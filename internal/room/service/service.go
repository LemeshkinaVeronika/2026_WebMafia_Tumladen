package service

import (
	"context"

	"github.com/webmafia/tumladan/internal/model"
)

type IRepository interface {
	Create(ctx context.Context, room *model.Room) error
	ListPublic(ctx context.Context) ([]model.Room, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.Room, error)
}
