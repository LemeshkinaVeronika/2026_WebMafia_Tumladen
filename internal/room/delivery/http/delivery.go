package http

import (
	"context"

	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
)

type IService interface {
	CreateRoom(ctx context.Context, actor model.Actor, req dto.CreateRoomRequest) (*dto.RoomResponse, error)
	ListPublicRooms(ctx context.Context) (*dto.ListPublicRoomsResponse, error)
	GetRoomByInviteCode(ctx context.Context, inviteCode string) (*dto.GetRoomByInviteCodeResponse, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{service: service}
}
