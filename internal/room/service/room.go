package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
)

const defaultOwnerActorID = "00000000-0000-0000-0000-000000000000"

type Service struct {
	repo IRepository
}

func New(repo IRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateRoom(ctx context.Context, req dto.CreateRoomRequest) (*dto.RoomResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrInvalidRoomName
	}

	now := time.Now().UTC()
	room := &model.Room{
		ID:           uuid.NewString(),
		Name:         name,
		IsPrivate:    false,
		OwnerActorID: defaultOwnerActorID,
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
