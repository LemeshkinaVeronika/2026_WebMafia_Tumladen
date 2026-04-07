package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	matchService "github.com/webmafia/tumladan/internal/match/service"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
	roomPostgres "github.com/webmafia/tumladan/internal/room/repository/postgres"
)

const (
	minRoomNameLength = 1
	maxRoomNameLength = 64

	defaultGameType   = "carcassonne"
	defaultMaxPlayers = 2

	carcassonneMinPlayers      = 2
	carcassonneMaxPlayers      = 5
	carcassonneDefaultTurnTime = 120
	carcassonneMinTurnTime     = 30
	carcassonneMaxTurnTime     = 300
)

//TODO: add ErrorOf + mapping

func roomToResponse(room model.Room) dto.RoomResponse {
	return dto.RoomResponse{
		ID:           room.ID,
		Name:         room.Name,
		IsPrivate:    room.IsPrivate,
		InviteCode:   room.InviteCode,
		OwnerActorID: room.OwnerActorID,
		Status:       string(room.Status),
		GameType:     room.GameType,
		MaxPlayers:   room.MaxPlayers,
		Settings:     json.RawMessage(room.Settings),
		PlayersCount: room.PlayersCount,
		CanStart:     room.Status == model.RoomStatusWaiting && room.PlayersCount >= 2,
		CreatedAt:    room.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    room.UpdatedAt.Format(time.RFC3339),
	}
}

func roomWithPlayerStatsToResponse(room model.Room, participants []model.RoomParticipant) dto.RoomResponse {
	resp := roomToResponse(room)
	resp.PlayersCount = len(participants)
	resp.CanStart = room.Status == model.RoomStatusWaiting && len(participants) >= 2
	return resp
}

func roomWithParticipantsToResponse(room model.Room, participants []model.RoomParticipant) dto.RoomResponse {
	resp := roomWithPlayerStatsToResponse(room, participants)
	resp.Participants = make([]dto.ParticipantResponse, 0, len(participants))

	for _, p := range participants {
		resp.Participants = append(resp.Participants, dto.ParticipantResponse{
			ActorID:     p.ActorID,
			DisplayName: p.DisplayName,
			JoinedAt:    p.JoinedAt.Format(time.RFC3339),
		})
	}

	return resp
}

func (s *Service) CreateRoom(ctx context.Context, req dto.CreateRoomServiceRequest) (*dto.RoomResponse, error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < minRoomNameLength || len(name) > maxRoomNameLength {
		return nil, ErrInvalidRoomName
	}

	now := time.Now().UTC()
	inviteCode := generateInviteCode()

	settings, err := normalizeRoomSettings(defaultGameType, nil)
	if err != nil {
		return nil, err
	}

	room := &model.Room{
		ID:           uuid.NewString(),
		Name:         name,
		IsPrivate:    false,
		InviteCode:   &inviteCode,
		OwnerActorID: req.Actor.ID,
		Status:       model.RoomStatusWaiting,
		GameType:     defaultGameType,
		MaxPlayers:   defaultMaxPlayers,
		Settings:     settings,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}

	resp := roomWithParticipantsToResponse(*room, nil)
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
		resp.Rooms = append(resp.Rooms, roomToResponse(room))
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
		Room: roomWithParticipantsToResponse(*room, participants),
	}

	return resp, nil
}

func (s *Service) JoinRoom(ctx context.Context, req dto.JoinRoomRequest) (*dto.RoomResponse, error) {
	room, participants, err := s.repo.JoinRoom(ctx, req.RoomID, req.Actor.ID, req.Actor.DisplayName)
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

	resp := roomWithParticipantsToResponse(*room, participants)
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

	resp := roomWithParticipantsToResponse(*room, participants)
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

	gameType := strings.TrimSpace(req.GameType)
	if gameType == "" {
		return nil, ErrInvalidGameType
	}

	maxPlayers := req.MaxPlayers
	if maxPlayers < 2 {
		return nil, ErrInvalidMaxPlayers
	}

	switch gameType {
	case "carcassonne":
		if maxPlayers < carcassonneMinPlayers || maxPlayers > carcassonneMaxPlayers {
			return nil, ErrInvalidMaxPlayers
		}
	default:
		return nil, ErrInvalidGameType
	}

	settings, err := normalizeRoomSettings(gameType, req.Settings)
	if err != nil {
		return nil, err
	}

	updatedRoom, participants, err := s.repo.UpdateSettings(ctx, req.RoomID, gameType, maxPlayers, settings)
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

	resp := roomWithParticipantsToResponse(*updatedRoom, participants)
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

	if len(participants) < 2 {
		return nil, ErrNotEnoughPlayers
	}

	match, matchPlayers, err := matchService.BuildInitialMatch(room, participants)
	if err != nil {
		return nil, err
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

	resp := roomWithParticipantsToResponse(*updatedRoom, updatedParticipants)
	return &resp, nil
}

// TODO:не забыть убрать заглушку
func (s *Service) FinishRoomMatch(ctx context.Context, req dto.FinishRoomMatchRequest) error {
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

	resultJSON, err := json.Marshal(map[string]any{
		"finished": true,
	})
	if err != nil {
		return err
	}

	_, _, err = s.repo.FinishActiveMatch(ctx, req.RoomID, model.JSONB(resultJSON))
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

func (s *Service) AbandonRoomMatch(ctx context.Context, roomID string, reason string) error {
	_, _, err := s.repo.AbandonActiveMatch(ctx, roomID, reason)
	if err != nil {
		switch {
		case errors.Is(err, roomPostgres.ErrActiveMatchNotFound):
			return ErrActiveMatchNotFound
		case errors.Is(err, roomPostgres.ErrNotFound):
			return ErrRoomNotFound
		default:
			return err
		}
	}

	return nil
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

	resp := roomWithParticipantsToResponse(*room, participants)
	return &resp, nil
}

func (s *Service) CleanupStaleRooms(ctx context.Context, waitingTTL, playingTTL time.Duration) error {
	now := time.Now().UTC()

	waitingCutoff := now.Add(-waitingTTL)
	playingCutoff := now.Add(-playingTTL)

	playingRooms, err := s.repo.FindStaleEmptyPlayingRooms(ctx, playingCutoff)
	if err != nil {
		return err
	}

	for _, room := range playingRooms {
		if err := s.AbandonRoomMatch(ctx, room.ID, "reconnect_timeout"); err != nil {
			if errors.Is(err, ErrActiveMatchNotFound) {
				continue
			}
			return err
		}
	}

	waitingRooms, err := s.repo.FindStaleEmptyWaitingRooms(ctx, waitingCutoff)
	if err != nil {
		return err
	}

	for _, room := range waitingRooms {
		if err := s.repo.DeleteRoom(ctx, room.ID); err != nil {
			if errors.Is(err, roomPostgres.ErrNotFound) {
				continue
			}
			return err
		}
	}

	return nil
}

func generateInviteCode() string {
	return strings.ToUpper(uuid.NewString()[:8])
}
