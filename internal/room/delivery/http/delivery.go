package http

import (
	"context"

	"github.com/webmafia/tumladan/internal/room/dto"
)

type IService interface {
	CreateRoom(ctx context.Context, req dto.CreateRoomServiceRequest) (*dto.RoomResponse, error)
	ListPublicRooms(ctx context.Context) (*dto.ListPublicRoomsResponse, error)
	GetRoomByInviteCode(ctx context.Context, inviteCode string) (*dto.GetRoomByInviteCodeResponse, error)

	JoinRoom(ctx context.Context, req dto.JoinRoomRequest) (*dto.RoomResponse, error)
	LeaveRoom(ctx context.Context, req dto.LeaveRoomRequest) error
	GetRoomState(ctx context.Context, roomID string) (*dto.RoomResponse, error)

	UpdateRoomSettings(ctx context.Context, req dto.UpdateRoomSettingsServiceRequest) (*dto.RoomResponse, error)
	StartRoom(ctx context.Context, req dto.StartRoomRequest) (*dto.RoomResponse, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{service: service}
}
