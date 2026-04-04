package service

import (
	"context"

	"github.com/webmafia/tumladan/internal/model"
)

type IRepository interface {
	Create(ctx context.Context, room *model.Room) error
	ListPublic(ctx context.Context) ([]model.Room, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.Room, error)
	GetByID(ctx context.Context, roomID string) (*model.Room, error)

	RemoveParticipant(ctx context.Context, roomID, actorID string) error
	ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipant, error)
	JoinRoom(ctx context.Context, roomID, actorID, displayName string) (*model.Room, []model.RoomParticipant, error)

	UpdateSettings(ctx context.Context, roomID, gameType string, maxPlayers int, settings model.JSONB) (*model.Room, []model.RoomParticipant, error)
	UpdateStatus(ctx context.Context, roomID string, status model.RoomStatus) error
}

type Service struct {
	repo IRepository
}

func New(repo IRepository) *Service {
	return &Service{repo: repo}
}
