package service

import (
	"context"
	"encoding/json"

	"github.com/webmafia/tumladan/internal/model"
)

type Engine interface {
	GameType() string

	NormalizeRoomSettings(raw json.RawMessage) (model.JSONB, error)
	ValidateRoomConfig(maxPlayers int, settings model.JSONB) error

	BuildInitialMatch(room *model.Room, participants []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error)

	ApplyAction(ctx context.Context, match *model.Match, players []model.MatchPlayer, req ApplyActionRequest) (ApplyActionResult, error)
	ApplyTurnTimeout(ctx context.Context, match *model.Match, players []model.MatchPlayer) (ApplyActionResult, error)
	BuildPublicState(match *model.Match, players []model.MatchPlayer) (json.RawMessage, error)
	BuildPrivateState(match *model.Match, players []model.MatchPlayer, actorID string) (json.RawMessage, error)
}

type ApplyActionRequest struct {
	ActorID string
	Action  string
	Payload json.RawMessage
}

type ApplyActionResult struct {
	NextState  model.JSONB
	NextStatus model.MatchStatus
	Result     *model.JSONB
}
