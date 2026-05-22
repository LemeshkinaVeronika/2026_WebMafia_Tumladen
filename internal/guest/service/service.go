package service

import (
	"context"
	"time"

	"github.com/webmafia/tumladan/internal/model"
	jwtprovider "github.com/webmafia/tumladan/pkg/jwt"
)

type IRepository interface {
	Create(ctx context.Context, session *model.GuestSession) error
	DeleteByActorID(ctx context.Context, actorID string, now time.Time) ([]string, error)
	CleanupExpired(ctx context.Context, now time.Time) ([]string, error)
}

type ITokenProvider interface {
	CreateGuestToken(ctx context.Context, sessionID, actorID, displayName string) (string, error)
	ParseToken(tokenStr string) (*jwtprovider.Claims, error)
}

type Service struct {
	repo          IRepository
	tokenProvider ITokenProvider
	tokenTTL      time.Duration
}

func New(repo IRepository, tokenProvider ITokenProvider, tokenTTL time.Duration) *Service {
	return &Service{
		repo:          repo,
		tokenProvider: tokenProvider,
		tokenTTL:      tokenTTL,
	}
}
