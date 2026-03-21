package http

import (
	"context"

	"github.com/webmafia/tumladan/internal/room/dto"
)

type IService interface {
	CreateRoom(ctx context.Context, req dto.CreateRoomRequest) (*dto.RoomResponse, error)
	ListPublicRooms(ctx context.Context) (*dto.ListPublicRoomsResponse, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{service: service}
}
