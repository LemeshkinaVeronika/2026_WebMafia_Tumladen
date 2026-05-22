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

//TODO: add ErrorOf + mapping

func (s *Service) CreateGuestSession(ctx context.Context, req dto.CreateGuestSessionRequest) (*dto.CreateGuestSessionResponse, error) {
	displayName := strings.TrimSpace(req.DisplayName)
	if len(displayName) < minDisplayNameLength || len(displayName) > maxDisplayNameLength {
		return nil, ErrInvalidDisplayName
	}

	if err := s.deletePreviousGuestSession(ctx, req.PreviousToken); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	session := &model.GuestSession{
		SessionID:   uuid.NewString(),
		ActorID:     uuid.NewString(),
		DisplayName: displayName,
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.CreateGuestToken(ctx, session.SessionID, session.ActorID, session.DisplayName)
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

func (s *Service) DeleteGuestSession(ctx context.Context, actorID string) error {
	if actorID == "" {
		return nil
	}

	_, err := s.repo.DeleteByActorID(ctx, actorID, time.Now().UTC())
	return err
}

func (s *Service) CleanupExpiredGuestSessions(ctx context.Context) ([]string, error) {
	return s.repo.CleanupExpired(ctx, time.Now().UTC())
}

func (s *Service) deletePreviousGuestSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	claims, err := s.tokenProvider.ParseToken(token)
	if err != nil {
		return nil
	}
	if claims.ActorType != string(model.ActorTypeGuest) || claims.ActorID == "" {
		return nil
	}

	return s.DeleteGuestSession(ctx, claims.ActorID)
}
