package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
	roomPostgres "github.com/webmafia/tumladan/internal/room/repository/postgres"
)

const (
	minRoomNameLength = 1
	maxRoomNameLength = 64
)

func (s *Service) CreateRoom(ctx context.Context, actor model.Actor, req dto.CreateRoomRequest) (*dto.RoomResponse, error) {
	name := strings.TrimSpace(req.Name)
	if len(name) < minRoomNameLength || len(name) > maxRoomNameLength {
		return nil, ErrInvalidRoomName
	}

	now := time.Now().UTC()
	inviteCode := generateInviteCode()

	room := &model.Room{
		ID:           uuid.NewString(),
		Name:         name,
		IsPrivate:    false,
		InviteCode:   &inviteCode,
		OwnerActorID: actor.ID,
		Status:       model.RoomStatusWaiting,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}

	resp := roomToResponse(*room)
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

	resp := &dto.GetRoomByInviteCodeResponse{
		Room: roomToResponse(*room),
	}

	return resp, nil
}

func roomToResponse(room model.Room) dto.RoomResponse {
	return dto.RoomResponse{
		ID:           room.ID,
		Name:         room.Name,
		IsPrivate:    room.IsPrivate,
		InviteCode:   room.InviteCode,
		OwnerActorID: room.OwnerActorID,
		Status:       string(room.Status),
		CreatedAt:    room.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    room.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *Service) JoinRoom(ctx context.Context, actor model.Actor, roomID string) (*dto.RoomResponse, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, roomPostgres.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	if err := s.repo.AddParticipant(ctx, roomID, actor.ID, actor.DisplayName); err != nil {
		return nil, err
	}

	participants, err := s.repo.ListParticipants(ctx, roomID)
	if err != nil {
		return nil, err
	}

	resp := roomWithParticipantsToResponse(*room, participants)
	return &resp, nil
}

func (s *Service) LeaveRoom(ctx context.Context, actor model.Actor, roomID string) error {
	return s.repo.RemoveParticipant(ctx, roomID, actor.ID)
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

func roomWithParticipantsToResponse(room model.Room, participants []model.RoomParticipantView) dto.RoomResponse {
	resp := roomToResponse(room)
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

func generateInviteCode() string {
	return strings.ToUpper(uuid.NewString()[:8])
}
