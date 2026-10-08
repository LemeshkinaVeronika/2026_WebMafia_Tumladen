package service

import (
	"context"

	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

type IRepository interface {
	Create(ctx context.Context, match *model.Match, players []model.MatchPlayer) error
	GetByID(ctx context.Context, matchID string) (*model.Match, []model.MatchPlayer, error)
	GetActiveByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error)
	GetLastByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error)
	ListRecentActivities(ctx context.Context, matchID string, maxStateVersion, limit int) ([]model.MatchActivity, error)
	GetActionReceipt(ctx context.Context, roomID, actorID, actionID string) (*model.MatchActionReceipt, error)
	SaveActionReceipt(ctx context.Context, receipt *model.MatchActionReceipt) error
	PersistActionResult(ctx context.Context, matchID string, expectedStateVersion int, state model.JSONB, status model.MatchStatus, result *model.JSONB, receipt *model.MatchActionReceipt, activities []model.MatchActivity) error
}

type MatchTerminator interface {
	FinishRoomMatch(ctx context.Context, req roomDTO.FinishRoomMatchRequest) error
}

type RoomLocker interface {
	Lock(ctx context.Context, roomID string) (context.Context, func(), error)
}

type Service struct {
	repo       IRepository
	terminator MatchTerminator
	games      *gameService.Facade
	roomLocks  RoomLocker
}

func NewWithRoomLocker(repo IRepository, terminator MatchTerminator, games *gameService.Facade, roomLocker RoomLocker) *Service {
	service := New(repo, terminator, games)
	if roomLocker != nil {
		service.roomLocks = roomLocker
	}
	return service
}

func New(repo IRepository, terminator MatchTerminator, games *gameService.Facade) *Service {
	return &Service{
		repo:       repo,
		terminator: terminator,
		games:      games,
		roomLocks:  newRoomLocks(),
	}
}
