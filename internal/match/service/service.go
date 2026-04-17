package service

import (
	"context"

	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

type IRepository interface {
	Create(ctx context.Context, match *model.Match, players []model.MatchPlayer) error
	GetByID(ctx context.Context, matchID string) (*model.Match, []model.MatchPlayer, error)
	GetActiveByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error)
	GetLastByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error)
	UpdateState(ctx context.Context, matchID string, state model.JSONB, status model.MatchStatus, result *model.JSONB) error
}

type MatchTerminator interface {
	FinishRoomMatch(ctx context.Context, req roomDTO.FinishRoomMatchRequest) error
}

type Service struct {
	repo       IRepository
	terminator MatchTerminator
}

func New(repo IRepository, terminator MatchTerminator) *Service {
	return &Service{
		repo:       repo,
		terminator: terminator,
	}
}
