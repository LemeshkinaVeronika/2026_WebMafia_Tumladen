package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/user/dto"
	"github.com/webmafia/tumladan/internal/user/tools"
)

const (
	minNicknameLength = 3
	maxNicknameLength = 32
	minPasswordLength = 8
)

func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	const op = "user.service.Register"

	nickname := strings.TrimSpace(req.Nickname)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if err := validateNickname(nickname); err != nil {
		return nil, fmt.Errorf("%s: %w", err, ErrValidation)
	}
	if err := validateEmail(email); err != nil {
		return nil, fmt.Errorf("%s: %w", err, ErrValidation)
	}
	if err := validatePassword(req.Password, req.PasswordConfirm); err != nil {
		return nil, fmt.Errorf("%s: %w", err, ErrValidation)
	}

	hash, err := tools.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to hash password: %w", op, err)
	}

	user := model.User{
		ID:           uuid.New(),
		Nickname:     nickname,
		Email:        email,
		PasswordHash: hash,
		AvatarURL:    "",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if req.Avatar != nil && s.storage != nil {
		objectName, err := s.storage.UploadAvatar(ctx, req.Avatar, req.AvatarFilename, req.AvatarSize, req.AvatarType)
		if err != nil {
			return nil, err
		}
		user.AvatarURL = objectName
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		if user.AvatarURL != "" && s.storage != nil {
			_ = s.storage.DeleteAvatar(ctx, user.AvatarURL)
		}
		return nil, mapRepositoryError(err)
	}

	if err := s.deletePreviousGuestSession(ctx, req.PreviousToken); err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.CreateUserToken(ctx, uuid.NewString(), user.ID.String(), user.Nickname)
	if err != nil {
		return nil, fmt.Errorf("[%s]: create token: %w", op, err)
	}

	return &dto.RegisterResponse{
		Actor: userActorResponse(user),
		Token: token,
	}, nil
}

func (s *Service) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	const op = "user.service.Login"

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" || req.Password == "" {
		return nil, fmt.Errorf("empty credentials: %w", ErrValidation)
	}

	var user *model.User
	var err error
	if strings.Contains(identifier, "@") {
		user, err = s.repo.GetUserByEmail(ctx, strings.ToLower(identifier))
	} else {
		user, err = s.repo.GetUserByLogin(ctx, identifier)
	}
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	if err := tools.Compare(user.PasswordHash, req.Password); err != nil {
		return nil, fmt.Errorf("[%s]: invalid credentials: %w", op, ErrValidation)
	}

	if err := s.deletePreviousGuestSession(ctx, req.PreviousToken); err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.CreateUserToken(ctx, uuid.NewString(), user.ID.String(), user.Nickname)
	if err != nil {
		return nil, fmt.Errorf("[%s]: create token: %w", op, err)
	}

	return &dto.LoginResponse{
		Actor: userActorResponse(*user),
		Token: token,
	}, nil
}

func (s *Service) deletePreviousGuestSession(ctx context.Context, token string) error {
	if s.guestSessionCleaner == nil || token == "" {
		return nil
	}

	claims, err := s.tokenProvider.ParseToken(token)
	if err != nil {
		return nil
	}
	if claims.ActorType != string(model.ActorTypeGuest) || claims.ActorID == "" {
		return nil
	}

	return s.guestSessionCleaner.DeleteGuestSession(ctx, claims.ActorID)
}

// Avatar Upload
func (s *Service) UploadAvatar(ctx context.Context, req dto.UploadAvatarRequest) (*dto.UploadAvatarResponse, error) {
	const op = "service.UpdateAvatar"

	objectName, err := s.storage.UploadAvatar(ctx, req.File, req.Filename, req.Size, req.ContentType)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateUserAvatar(ctx, req.UserID, objectName); err != nil {
		if delErr := s.storage.DeleteAvatar(ctx, objectName); delErr != nil {
			return nil, fmt.Errorf("[%s]: failed to delete uploaded avatar %q): %w", op, objectName, delErr)
		}
		return nil, mapRepositoryError(err)
	}

	return &dto.UploadAvatarResponse{URL: objectName}, nil
}

func (s *Service) DeleteAvatar(ctx context.Context, req dto.DeleteAvatarRequest) error {
	const op = "service.DeleteAvatar"

	user, err := s.repo.GetUserByID(ctx, req.UserID)
	if err != nil {
		return mapRepositoryError(err)
	}

	if user.AvatarURL != "" {
		if err := s.storage.DeleteAvatar(ctx, user.AvatarURL); err != nil {
			return fmt.Errorf("[%s]: delete avatar from storage: %w", op, err)
		}
		if err := s.repo.UpdateUserAvatar(ctx, req.UserID, ""); err != nil {
			return mapRepositoryError(err)
		}

	}

	return nil
}

func (s *Service) UpdateProfile(ctx context.Context, req dto.UpdateProfileRequest) (*dto.UpdateProfileResponse, error) {
	const op = "user.service.UpdateProfile"

	id, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("[%s]: invalid user ID: %w", op, err)
	}

	nickname := strings.TrimSpace(req.Nickname)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if err := validateNickname(nickname); err != nil {
		return nil, fmt.Errorf("%s: %w", err, ErrValidation)
	}
	if err := validateEmail(email); err != nil {
		return nil, fmt.Errorf("%s: %w", err, ErrValidation)
	}

	user := model.User{
		ID:       id,
		Nickname: nickname,
		Email:    email,
	}

	if req.Password != "" {
		if err := validatePassword(req.Password, req.PasswordConfirm); err != nil {
			return nil, fmt.Errorf("%s: %w", err, ErrValidation)
		}
		hash, err := tools.Hash(req.Password)
		if err != nil {
			return nil, fmt.Errorf("[%s]: failed to hash password: %w", op, err)
		}
		user.PasswordHash = hash
	}

	if err := s.repo.UpdateUserProfile(ctx, user); err != nil {
		return nil, mapRepositoryError(err)
	}

	updatedUser, err := s.repo.GetUserByID(ctx, user.ID.String())
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	profile, err := s.buildProfileResponse(ctx, *updatedUser)
	if err != nil {
		return nil, err
	}

	return &dto.UpdateProfileResponse{
		ID:           profile.ID,
		Nickname:     profile.Nickname,
		Email:        profile.Email,
		AvatarURL:    profile.AvatarURL,
		CurrentRoom:  profile.CurrentRoom,
		Achievements: profile.Achievements,
		MatchHistory: profile.MatchHistory,
		Stats:        profile.Stats,
	}, nil
}

func (s *Service) GetProfile(ctx context.Context, req dto.GetProfileRequest) (*dto.GetProfileResponse, error) {
	if _, err := uuid.Parse(req.UserID); err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", ErrValidation)
	}

	user, err := s.repo.GetUserByID(ctx, req.UserID)
	if err != nil {
		return nil, mapRepositoryError(err)
	}

	return s.buildProfileResponse(ctx, *user)
}

func (s *Service) buildProfileResponse(ctx context.Context, user model.User) (*dto.GetProfileResponse, error) {
	currentRoom, err := s.repo.GetCurrentRoomByUserID(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	matches, err := s.repo.ListFinishedMatchesByUserID(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	achievements, err := s.repo.ListAchievementsByUserID(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}
	gameStats, err := s.repo.ListGameStatsByUserID(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	history := make([]dto.MatchHistoryItem, 0, len(matches))
	for _, match := range matches {
		item := matchHistoryItem(match)
		history = append(history, item)
	}

	return &dto.GetProfileResponse{
		ID:           user.ID.String(),
		Nickname:     user.Nickname,
		Email:        user.Email,
		AvatarURL:    user.AvatarURL,
		CurrentRoom:  currentRoomResponse(currentRoom),
		Achievements: achievementResponses(achievements),
		MatchHistory: history,
		Stats:        projectedStatsSummary(gameStats),
	}, nil
}

func projectedStatsSummary(rows []model.UserGameStats) dto.UserGameStatsSummary {
	summary := dto.UserGameStatsSummary{ByGame: make([]dto.UserGameStats, 0, len(rows))}
	for _, row := range rows {
		stats := dto.UserGameStats{
			GameType:   row.GameType,
			Matches:    row.Matches,
			Wins:       row.Wins,
			Losses:     row.Losses,
			Draws:      row.Draws,
			TotalScore: row.TotalScore,
			BestScore:  row.BestScore,
		}
		normalizeStats(&stats)
		summary.ByGame = append(summary.ByGame, stats)
		summary.Overall.Matches += row.Matches
		summary.Overall.Wins += row.Wins
		summary.Overall.Losses += row.Losses
		summary.Overall.Draws += row.Draws
		summary.Overall.TotalScore += row.TotalScore
		if row.BestScore > summary.Overall.BestScore {
			summary.Overall.BestScore = row.BestScore
		}
	}
	normalizeStats(&summary.Overall)
	return summary
}

func currentRoomResponse(room *model.CurrentRoom) *dto.CurrentRoomResponse {
	if room == nil {
		return nil
	}
	return &dto.CurrentRoomResponse{
		ID:         room.ID,
		Name:       room.Name,
		InviteCode: room.InviteCode,
		Status:     string(room.Status),
		GameType:   room.GameType,
		MatchID:    room.MatchID,
	}
}

func achievementResponses(achievements []model.Achievement) []dto.Achievement {
	resp := make([]dto.Achievement, 0, len(achievements))
	for _, achievement := range achievements {
		var unlockedAt *string
		if achievement.UnlockedAt != nil {
			value := achievement.UnlockedAt.Format(time.RFC3339)
			unlockedAt = &value
		}

		resp = append(resp, dto.Achievement{
			Code:        achievement.Code,
			Title:       achievement.Title,
			Description: achievement.Description,
			GameType:    achievement.GameType,
			UnlockedAt:  unlockedAt,
			MatchID:     achievement.MatchID,
		})
	}
	return resp
}

type storedMatchResult struct {
	Winners     []string `json:"winners"`
	FinalScores []struct {
		ActorID string `json:"actorId"`
		Score   int    `json:"score"`
	} `json:"finalScores"`
}

func matchHistoryItem(matchWithPlayers model.MatchWithPlayers) dto.MatchHistoryItem {
	match := matchWithPlayers.Match
	result := parseStoredMatchResult(match.Result)
	scoreByActorID := make(map[string]int, len(result.FinalScores))
	for _, score := range result.FinalScores {
		scoreByActorID[score.ActorID] = score.Score
	}

	players := make([]dto.MatchHistoryPlayerScore, 0, len(matchWithPlayers.Players))
	winners := make(map[string]struct{}, len(result.Winners))
	for _, actorID := range result.Winners {
		winners[actorID] = struct{}{}
	}

	for _, player := range matchWithPlayers.Players {
		score := scoreByActorID[player.ActorID]
		_, isWinner := winners[player.ActorID]
		players = append(players, dto.MatchHistoryPlayerScore{
			ActorID:     player.ActorID,
			ActorType:   string(player.ActorType),
			DisplayName: player.DisplayName,
			Seat:        player.Seat,
			Score:       score,
			IsWinner:    isWinner,
		})
	}
	assignRanks(players)

	var terminationReason *string
	if match.TerminationReason != nil {
		reason := string(*match.TerminationReason)
		terminationReason = &reason
	}

	var terminatedAt *string
	if match.TerminatedAt != nil {
		value := match.TerminatedAt.Format(time.RFC3339)
		terminatedAt = &value
	}

	return dto.MatchHistoryItem{
		ID:                  match.ID,
		RoomID:              match.RoomID,
		GameType:            match.GameType,
		Status:              string(match.Status),
		TerminationReason:   terminationReason,
		TerminatedByActorID: match.TerminatedByActorID,
		TerminatedAt:        terminatedAt,
		Result: dto.MatchResultSummary{
			HasResult: len(result.FinalScores) > 0,
			Winners:   result.Winners,
		},
		Players:   players,
		CreatedAt: match.CreatedAt.Format(time.RFC3339),
		UpdatedAt: match.UpdatedAt.Format(time.RFC3339),
	}
}

func parseStoredMatchResult(raw *model.JSONB) storedMatchResult {
	if raw == nil || len(*raw) == 0 || string(*raw) == "null" {
		return storedMatchResult{}
	}

	var result storedMatchResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return storedMatchResult{}
	}
	return result
}

func assignRanks(players []dto.MatchHistoryPlayerScore) {
	sorted := make([]dto.MatchHistoryPlayerScore, len(players))
	copy(sorted, players)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score == sorted[j].Score {
			return sorted[i].Seat < sorted[j].Seat
		}
		return sorted[i].Score > sorted[j].Score
	})

	rankByActorID := make(map[string]int, len(players))
	lastScore := 0
	lastRank := 0
	for i, player := range sorted {
		rank := i + 1
		if i > 0 && player.Score == lastScore {
			rank = lastRank
		}
		rankByActorID[player.ActorID] = rank
		lastScore = player.Score
		lastRank = rank
	}

	for i := range players {
		players[i].Rank = rankByActorID[players[i].ActorID]
	}
}

func normalizeStats(stats *dto.UserGameStats) {
	if stats.Matches == 0 {
		return
	}
	stats.WinRate = float64(stats.Wins) / float64(stats.Matches)
	stats.AverageScore = float64(stats.TotalScore) / float64(stats.Matches)
}

func validateNickname(nickname string) error {
	length := utf8.RuneCountInString(nickname)
	if length < minNicknameLength || length > maxNicknameLength {
		return fmt.Errorf("nickname must be between %d and %d characters", minNicknameLength, maxNicknameLength)
	}
	for _, r := range nickname {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return errors.New("nickname can contain only letters, digits, underscore or hyphen")
	}
	return nil
}

func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email")
	}
	return nil
}

func validatePassword(password, confirmation string) error {
	if password != confirmation {
		return errors.New("password confirmation does not match")
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must contain at least %d characters", minPasswordLength)
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("password must contain uppercase letters, lowercase letters and digits")
	}
	return nil
}

func userActorResponse(user model.User) dto.ActorResponse {
	return dto.ActorResponse{
		ID:          user.ID.String(),
		Type:        string(model.ActorTypeUser),
		DisplayName: user.Nickname,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
	}
}
