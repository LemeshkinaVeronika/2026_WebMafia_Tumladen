package service

import (
	"context"
	"time"

	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

type IRepository interface {
	Create(ctx context.Context, room *model.Room) error
	ListPublic(ctx context.Context) ([]model.Room, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.Room, error)
	GetByID(ctx context.Context, roomID string) (*model.Room, error)

	RemoveParticipant(ctx context.Context, roomID, actorID string) error
	ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipant, error)
	JoinRoom(ctx context.Context, roomID, actorID string, actorType model.ActorType, displayName string, reservedSlots int) (*model.Room, []model.RoomParticipant, error)

	UpdateSettings(ctx context.Context, roomID, name string, isPrivate bool, gameType string, maxPlayers int, settings model.JSONB, reservedSlots int) (*model.Room, []model.RoomParticipant, error)
	UpdateStatus(ctx context.Context, roomID string, status model.RoomStatus) error

	StartRoomWithMatch(ctx context.Context, roomID string, match *model.Match, players []model.MatchPlayer) (*model.Room, []model.RoomParticipant, error)

	TerminateActiveMatch(ctx context.Context, roomID string, state *model.JSONB, reason model.MatchTerminationReason, result *model.JSONB, terminatedByActorID *string, terminatedAt time.Time) (*model.Match, []model.MatchPlayer, error)
	MarkActiveMatchPlayerDisconnected(ctx context.Context, roomID, actorID string, disconnectedAt time.Time) error
	MarkActiveMatchPlayerConnected(ctx context.Context, roomID, actorID string) error
	FindRoomsWithReconnectTimeout(ctx context.Context, olderThan time.Time) ([]model.Room, error)
	DeleteRoom(ctx context.Context, roomID string) error

	KickParticipant(ctx context.Context, roomID, targetActorID string) error

	FindStaleEmptyWaitingRooms(ctx context.Context, olderThan time.Time) ([]model.Room, error)

	MarkRoomEmpty(ctx context.Context, roomID string) error
}

type Service struct {
	repo  IRepository
	games *gameService.Facade
}

func New(repo IRepository, games *gameService.Facade) *Service {
	return &Service{
		repo:  repo,
		games: games,
	}
}
