package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/guest/dto"
	"github.com/webmafia/tumladan/internal/model"
)

const (
	minDisplayNameLength = 2
	maxDisplayNameLength = 32
)

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

func (s *Service) CreateGuestSession(ctx context.Context, req dto.CreateGuestSessionRequest) (*dto.CreateGuestSessionResponse, error) {
	displayName := strings.TrimSpace(req.DisplayName)
	if len(displayName) < minDisplayNameLength || len(displayName) > maxDisplayNameLength {
		return nil, ErrInvalidDisplayName
	}

	session := &model.GuestSession{
		ActorID:     uuid.NewString(),
		DisplayName: displayName,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.CreateGuestToken(ctx, session.ActorID, session.DisplayName)
	if err != nil {
		return nil, err
	}

	return &dto.CreateGuestSessionResponse{
		Actor: dto.GuestActorResponse{
			ID:          session.ActorID,
			Type:        "guest",
			DisplayName: session.DisplayName,
		},
		Token: token,
	}, nil
}
