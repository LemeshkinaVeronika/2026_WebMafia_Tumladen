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

	JoinRoom(ctx context.Context, actor model.Actor, roomID string) (*dto.RoomResponse, error)
	LeaveRoom(ctx context.Context, actor model.Actor, roomID string) error
	GetRoomState(ctx context.Context, roomID string) (*dto.RoomResponse, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{service: service}
}
