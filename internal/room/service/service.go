package service

import (
	"context"
	"github.com/webmafia/tumladan/internal/model"
	"time"
)

type IRepository interface {
	Create(ctx context.Context, room *model.Room) error
	ListPublic(ctx context.Context) ([]model.Room, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.Room, error)
	GetByID(ctx context.Context, roomID string) (*model.Room, error)

	RemoveParticipant(ctx context.Context, roomID, actorID string) error
	ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipant, error)
	JoinRoom(ctx context.Context, roomID, actorID, displayName string) (*model.Room, []model.RoomParticipant, error)

	UpdateSettings(ctx context.Context, roomID, name, gameType string, maxPlayers int, settings model.JSONB) (*model.Room, []model.RoomParticipant, error)
	UpdateStatus(ctx context.Context, roomID string, status model.RoomStatus) error

	StartRoomWithMatch(ctx context.Context, roomID string, match *model.Match, players []model.MatchPlayer) (*model.Room, []model.RoomParticipant, error)

	FinishActiveMatch(ctx context.Context, roomID string, result model.JSONB) (*model.Match, []model.MatchPlayer, error)
	AbandonActiveMatch(ctx context.Context, roomID string, reason string) (*model.Match, []model.MatchPlayer, error)
	DeleteRoom(ctx context.Context, roomID string) error

	KickParticipant(ctx context.Context, roomID, targetActorID string) error

	FindStaleEmptyWaitingRooms(ctx context.Context, olderThan time.Time) ([]model.Room, error)
	FindStaleEmptyPlayingRooms(ctx context.Context, olderThan time.Time) ([]model.Room, error)
}

type Service struct {
	repo IRepository
}

func New(repo IRepository) *Service {
	return &Service{repo: repo}
}
