package service

import (
	"context"

	"github.com/webmafia/tumladan/internal/model"
)

type IRepository interface {
	Create(ctx context.Context, session *model.GuestSession) error
}

type ITokenProvider interface {
	CreateGuestToken(ctx context.Context, actorID, displayName string) (string, error)
}

type Service struct {
	repo          IRepository
	tokenProvider ITokenProvider
}

func New(repo IRepository, tokenProvider ITokenProvider) *Service {
	return &Service{
		repo:          repo,
		tokenProvider: tokenProvider,
	}
}
