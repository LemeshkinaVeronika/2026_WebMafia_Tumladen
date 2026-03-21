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

	AddParticipant(ctx context.Context, roomID, actorID, displayName string) error
	RemoveParticipant(ctx context.Context, roomID, actorID string) error
	ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipantView, error)
}

type Service struct {
	repo IRepository
}

func New(repo IRepository) *Service {
	return &Service{repo: repo}
}
