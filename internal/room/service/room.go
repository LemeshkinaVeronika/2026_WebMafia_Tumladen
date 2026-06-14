package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
	roomPostgres "github.com/webmafia/tumladan/internal/room/repository/postgres"
)

const (
	minRoomNameLength = 1
	maxRoomNameLength = 64

	defaultGameType   = "carcassonne"
	defaultMaxPlayers = 2
)

func (s *Service) roomToResponse(room model.Room) dto.RoomResponse {
	bots := s.roomBotParticipants(room)
	playersCount := room.PlayersCount + len(bots)
	canStart := room.Status == model.RoomStatusWaiting && room.PlayersCount > 0 && playersCount >= 2
	return dto.RoomResponse{
		ID:             room.ID,
		Name:           room.Name,
		IsPrivate:      room.IsPrivate,
		InviteCode:     room.InviteCode,
		OwnerActorID:   room.OwnerActorID,
		OwnerActorType: string(room.OwnerActorType),
		Status:         string(room.Status),
		GameType:       room.GameType,
		MaxPlayers:     room.MaxPlayers,
		Settings:       json.RawMessage(room.Settings),
		PlayersCount:   playersCount,
		CanStart:       canStart,
		CreatedAt:      room.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      room.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *Service) roomWithPlayerStatsToResponse(room model.Room, participants []model.RoomParticipant) dto.RoomResponse {
	resp := s.roomToResponse(room)
	resp.PlayersCount = len(participants) + len(s.roomBotParticipants(room))
	resp.CanStart = room.Status == model.RoomStatusWaiting && len(participants) > 0 && resp.PlayersCount >= 2
	return resp
}

func (s *Service) roomWithParticipantsToResponse(room model.Room, participants []model.RoomParticipant) dto.RoomResponse {
	allParticipants := s.withRoomBotParticipants(room, participants)
	resp := s.roomWithPlayerStatsToResponse(room, participants)
	resp.Participants = make([]dto.ParticipantResponse, 0, len(allParticipants))

	for _, p := range allParticipants {
		resp.Participants = append(resp.Participants, dto.ParticipantResponse{
			ActorID:       p.ActorID,
			ActorType:     string(p.ActorType),
			DisplayName:   p.DisplayName,
			BotDifficulty: p.BotDifficulty,
			JoinedAt:      p.JoinedAt.Format(time.RFC3339),
		})
	}

	return resp
}

func (s *Service) withRoomBotParticipants(room model.Room, participants []model.RoomParticipant) []model.RoomParticipant {
	bots := s.roomBotParticipants(room)
	if len(bots) == 0 {
		return participants
	}
	allParticipants := make([]model.RoomParticipant, 0, len(participants)+len(bots))
	allParticipants = append(allParticipants, participants...)
	allParticipants = append(allParticipants, bots...)
	return allParticipants
}

func (s *Service) roomBotParticipants(room model.Room) []model.RoomParticipant {
	if s == nil || s.games == nil {
		return nil
	}
	bots, err := s.games.BuildRoomBotParticipants(room.GameType, room.ID, room.Settings, room.CreatedAt)
	if err != nil {
		return nil
	}
	return bots
}

func (s *Service) roomReservedSlots(room model.Room) int {
	return len(s.roomBotParticipants(room))
}

func (s *Service) CreateRoom(ctx context.Context, req dto.CreateRoomServiceRequest) (*dto.RoomResponse, error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < minRoomNameLength || len(name) > maxRoomNameLength {
		return nil, ErrInvalidRoomName
	}

	now := time.Now().UTC()
	inviteCode := generateInviteCode()

	settings, err := s.games.NormalizeRoomSettings(defaultGameType, nil)
	if err != nil {
		return nil, mapGameRoomError(err)
	}
	if err := s.games.ValidateRoomConfig(defaultGameType, defaultMaxPlayers, settings); err != nil {
		return nil, mapGameRoomError(err)
	}

	room := &model.Room{
		ID:             uuid.NewString(),
		Name:           name,
		IsPrivate:      req.IsPrivate,
		InviteCode:     &inviteCode,
		OwnerActorID:   req.Actor.ID,
		OwnerActorType: model.ActorType(req.Actor.Type),
		Status:         model.RoomStatusWaiting,
		GameType:       defaultGameType,
		MaxPlayers:     defaultMaxPlayers,
		Settings:       settings,
		LastEmptyAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}

	resp := s.roomWithParticipantsToResponse(*room, nil)
	return &resp, nil
}

func (s *Service) ListPublicRooms(ctx context.Context) (*dto.ListPublicRoomsResponse, error) {
	rooms, err := s.repo.ListPublic(ctx)
	if err != nil {
		return nil, err
	}

	resp := &dto.ListPublicRoomsResponse{
		Rooms: make([]dto.RoomResponse, 0, len(rooms)),
	}

	for _, room := range rooms {
		resp.Rooms = append(resp.Rooms, s.roomToResponse(room))
	}

	return resp, nil
}

func (s *Service) GetRoomByInviteCode(ctx context.Context, inviteCode string) (*dto.GetRoomByInviteCodeResponse, error) {
	code := strings.TrimSpace(inviteCode)
	if code == "" {
		return nil, ErrRoomNotFound
	}

	room, err := s.repo.GetByInviteCode(ctx, code)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	participants, err := s.repo.ListParticipants(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	resp := &dto.GetRoomByInviteCodeResponse{
		Room: s.roomWithParticipantsToResponse(*room, participants),
	}

	return resp, nil
}

func (s *Service) JoinRoom(ctx context.Context, req dto.JoinRoomRequest) (*dto.RoomResponse, error) {
	currentRoom, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	room, participants, err := s.repo.JoinRoom(
		ctx,
		req.RoomID,
		req.Actor.ID,
		model.ActorType(req.Actor.Type),
		req.Actor.DisplayName,
		s.roomReservedSlots(*currentRoom),
	)
	if err != nil {
		switch {
		case errors.Is(err, roomPostgres.ErrNotFound):
			return nil, ErrRoomNotFound
		case errors.Is(err, roomPostgres.ErrRoomFull):
			return nil, ErrRoomFull
		case errors.Is(err, roomPostgres.ErrRoomNotJoinable):
			return nil, ErrRoomNotJoinable
		default:
			return nil, err
		}
	}

	resp := s.roomWithParticipantsToResponse(*room, participants)
	return &resp, nil
}

func (s *Service) LeaveRoom(ctx context.Context, req dto.LeaveRoomRequest) error {
	return s.repo.RemoveParticipant(ctx, req.RoomID, req.ActorID)
}

func (s *Service) GetRoomState(ctx context.Context, roomID string) (*dto.RoomResponse, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	participants, err := s.repo.ListParticipants(ctx, roomID)
	if err != nil {
		return nil, err
	}

	resp := s.roomWithParticipantsToResponse(*room, participants)
	return &resp, nil
}

func (s *Service) UpdateRoomSettings(ctx context.Context, req dto.UpdateRoomSettingsServiceRequest) (*dto.RoomResponse, error) {
	room, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	if room.OwnerActorID != req.ActorID {
		return nil, ErrForbidden
	}

	if room.Status != model.RoomStatusWaiting {
		return nil, ErrRoomSettingsLocked
	}

	name := room.Name
	if req.Name != "" {
		name = strings.TrimSpace(req.Name)
		if len(name) < minRoomNameLength || len(name) > maxRoomNameLength {
			return nil, ErrInvalidRoomName
		}
	}

	isPrivate := room.IsPrivate
	if req.IsPrivate != nil {
		isPrivate = *req.IsPrivate
	}

	gameType := room.GameType
	if req.GameType != "" {
		gameType = strings.TrimSpace(req.GameType)
		if gameType == "" {
			return nil, ErrInvalidGameType
		}
	}

	maxPlayers := room.MaxPlayers
	if req.MaxPlayers != 0 {
		maxPlayers = req.MaxPlayers
	}
	if maxPlayers < 2 {
		return nil, ErrInvalidMaxPlayers
	}

	settingsRaw := req.Settings
	if len(settingsRaw) == 0 {
		settingsRaw = json.RawMessage(room.Settings)
	}

	settings, err := s.games.NormalizeRoomSettings(gameType, settingsRaw)
	if err != nil {
		return nil, mapGameRoomError(err)
	}

	if err := s.games.ValidateRoomConfig(gameType, maxPlayers, settings); err != nil {
		return nil, mapGameRoomError(err)
	}

	currentParticipants, err := s.repo.ListParticipants(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}
	bots, err := s.games.BuildRoomBotParticipants(gameType, req.RoomID, settings, room.CreatedAt)
	if err != nil {
		return nil, mapGameRoomError(err)
	}
	if len(currentParticipants)+len(bots) > maxPlayers {
		return nil, ErrMaxPlayersLessThanParticipants
	}

	updatedRoom, participants, err := s.repo.UpdateSettings(ctx, req.RoomID, name, isPrivate, gameType, maxPlayers, settings, len(bots))
	if err != nil {
		switch {
		case errors.Is(err, roomPostgres.ErrNotFound):
			return nil, ErrRoomNotFound
		case errors.Is(err, roomPostgres.ErrMaxPlayersLessThanParticipants):
			return nil, ErrMaxPlayersLessThanParticipants
		case errors.Is(err, roomPostgres.ErrRoomSettingsLocked):
			return nil, ErrRoomSettingsLocked
		default:
			return nil, err
		}
	}

	resp := s.roomWithParticipantsToResponse(*updatedRoom, participants)
	return &resp, nil
}

func (s *Service) StartRoom(ctx context.Context, req dto.StartRoomRequest) (*dto.RoomResponse, error) {
	room, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	if room.OwnerActorID != req.ActorID {
		return nil, ErrForbidden
	}

	if room.Status != model.RoomStatusWaiting {
		return nil, ErrRoomNotReady
	}

	participants, err := s.repo.ListParticipants(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	if len(participants) == 0 {
		return nil, ErrNotEnoughPlayers
	}
	allParticipants := s.withRoomBotParticipants(*room, participants)
	if len(allParticipants) < 2 {
		return nil, ErrNotEnoughPlayers
	}
	if len(allParticipants) > room.MaxPlayers {
		return nil, ErrRoomNotReady
	}

	match, matchPlayers, err := s.games.BuildInitialMatch(room, allParticipants)
	if err != nil {
		return nil, mapGameRoomError(err)
	}

	updatedRoom, updatedParticipants, err := s.repo.StartRoomWithMatch(ctx, req.RoomID, match, matchPlayers)
	if err != nil {
		switch {
		case errors.Is(err, roomPostgres.ErrNotFound):
			return nil, ErrRoomNotFound
		case errors.Is(err, roomPostgres.ErrRoomNotReady):
			return nil, ErrRoomNotReady
		case errors.Is(err, roomPostgres.ErrNotEnoughPlayers):
			return nil, ErrNotEnoughPlayers
		default:
			return nil, err
		}
	}

	resp := s.roomWithParticipantsToResponse(*updatedRoom, updatedParticipants)
	return &resp, nil
}

func mapGameRoomError(err error) error {
	switch {
	case errors.Is(err, gameService.ErrUnsupportedGameType):
		return ErrInvalidGameType
	case errors.Is(err, gameService.ErrInvalidRoomConfig):
		return ErrInvalidMaxPlayers
	case errors.Is(err, gameService.ErrInvalidRoomSettings):
		return ErrInvalidRoomSettings
	default:
		return err
	}
}

func (s *Service) FinishRoomMatch(ctx context.Context, req dto.FinishRoomMatchRequest) error {
	room, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return ErrRoomNotFound
		}
		return err
	}

	reason, err := normalizeMatchTerminationReason(req.Reason)
	if err != nil {
		return err
	}

	if req.ActorID != nil {
		participants, participantsErr := s.repo.ListParticipants(ctx, req.RoomID)
		if participantsErr != nil {
			return participantsErr
		}
		if !containsParticipant(s.withRoomBotParticipants(*room, participants), *req.ActorID) {
			return ErrForbidden
		}
	}

	var result *model.JSONB
	if len(req.Result) > 0 && string(req.Result) != "null" {
		raw := model.JSONB(req.Result)
		result = &raw
	}

	var gameState *model.JSONB
	if len(req.GameState) > 0 && string(req.GameState) != "null" {
		raw := model.JSONB(req.GameState)
		gameState = &raw
	}

	terminatedAt := time.Now().UTC()

	_, _, err = s.repo.TerminateActiveMatch(ctx, req.RoomID, gameState, reason, result, req.ActorID, terminatedAt)
	if err != nil {
		switch {
		case errors.Is(err, roomPostgres.ErrNotFound):
			return ErrRoomNotFound
		case errors.Is(err, roomPostgres.ErrActiveMatchNotFound):
			return ErrActiveMatchNotFound
		default:
			return err
		}
	}

	return nil
}

func (s *Service) MarkActorDisconnectedInActiveMatch(ctx context.Context, roomID, actorID string) error {
	return s.repo.MarkActiveMatchPlayerDisconnected(ctx, roomID, actorID, time.Now().UTC())
}

func (s *Service) MarkActorReconnectedInActiveMatch(ctx context.Context, roomID, actorID string) error {
	return s.repo.MarkActiveMatchPlayerConnected(ctx, roomID, actorID)
}

func (s *Service) LeaveActiveMatch(ctx context.Context, roomID, actorID string) error {
	return s.FinishRoomMatch(ctx, dto.FinishRoomMatchRequest{
		ActorID: &actorID,
		RoomID:  roomID,
		Reason:  string(model.MatchTerminationReasonPlayerLeft),
	})
}

func (s *Service) TerminateRoomMatch(ctx context.Context, roomID string, reason model.MatchTerminationReason) error {
	return s.FinishRoomMatch(ctx, dto.FinishRoomMatchRequest{
		RoomID: roomID,
		Reason: string(reason),
	})
}

func normalizeMatchTerminationReason(reason string) (model.MatchTerminationReason, error) {
	switch model.MatchTerminationReason(strings.TrimSpace(reason)) {
	case model.MatchTerminationReasonNormalCompletion,
		model.MatchTerminationReasonPlayerLeft,
		model.MatchTerminationReasonReconnectTimeout,
		model.MatchTerminationReasonRoomDeleted:
		return model.MatchTerminationReason(strings.TrimSpace(reason)), nil
	default:
		return "", ErrInvalidMatchTerminationReason
	}
}

func containsParticipant(participants []model.RoomParticipant, actorID string) bool {
	for _, participant := range participants {
		if participant.ActorID == actorID {
			return true
		}
	}

	return false
}

func (s *Service) DeleteRoom(ctx context.Context, req dto.DeleteRoomRequest) error {
	room, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return ErrRoomNotFound
		}
		return err
	}

	if room.OwnerActorID != req.ActorID {
		return ErrForbidden
	}

	if err := s.repo.DeleteRoom(ctx, req.RoomID); err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return ErrRoomNotFound
		}
		return err
	}

	return nil
}

func (s *Service) KickParticipant(ctx context.Context, req dto.KickParticipantRequest) (*dto.RoomResponse, error) {
	room, err := s.repo.GetByID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	if room.OwnerActorID != req.ActorID {
		return nil, ErrForbidden
	}

	if room.Status != model.RoomStatusWaiting {
		return nil, ErrRoomModerationLocked
	}

	if req.TargetActorID == req.ActorID {
		return nil, ErrCannotKickYourself
	}

	participants, err := s.repo.ListParticipants(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, participant := range participants {
		if participant.ActorID == req.TargetActorID {
			found = true
			break
		}
	}

	if !found {
		return nil, ErrParticipantNotFound
	}

	if err := s.repo.KickParticipant(ctx, req.RoomID, req.TargetActorID); err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	participants, err = s.repo.ListParticipants(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}

	room.PlayersCount = len(participants)

	resp := s.roomWithParticipantsToResponse(*room, participants)
	return &resp, nil
}

func (s *Service) CleanupStaleRooms(ctx context.Context, waitingTTL, playingTTL time.Duration) ([]string, error) {
	now := time.Now().UTC()

	waitingCutoff := now.Add(-waitingTTL)
	playingCutoff := now.Add(-playingTTL)

	playingRooms, err := s.repo.FindRoomsWithReconnectTimeout(ctx, playingCutoff)
	if err != nil {
		return nil, err
	}

	terminatedRoomIDs := make([]string, 0, len(playingRooms))
	for _, room := range playingRooms {
		if err := s.TerminateRoomMatch(ctx, room.ID, model.MatchTerminationReasonReconnectTimeout); err != nil {
			if errors.Is(err, ErrActiveMatchNotFound) {
				continue
			}
			return nil, err
		}
		terminatedRoomIDs = append(terminatedRoomIDs, room.ID)
	}

	waitingRooms, err := s.repo.FindStaleEmptyWaitingRooms(ctx, waitingCutoff)
	if err != nil {
		return nil, err
	}

	for _, room := range waitingRooms {
		if err := s.repo.DeleteRoom(ctx, room.ID); err != nil {
			if errors.Is(err, roomPostgres.ErrNotFound) {
				continue
			}
			if errors.Is(err, roomPostgres.ErrForbiddenDeleteActiveRoom) {
				continue
			}
			return nil, err
		}
	}

	return terminatedRoomIDs, nil
}

func (s *Service) MarkRoomEmpty(ctx context.Context, roomID string) error {
	return s.repo.MarkRoomEmpty(ctx, roomID)
}

func generateInviteCode() string {
	return strings.ToUpper(uuid.NewString()[:8])
}
