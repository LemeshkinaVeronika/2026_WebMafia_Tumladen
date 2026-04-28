package service

import (
	"context"
	"encoding/json"

	"github.com/webmafia/tumladan/internal/model"
)

type Facade struct {
	registry *Registry
}

func NewFacade(registry *Registry) *Facade {
	return &Facade{registry: registry}
}

func (f *Facade) NormalizeRoomSettings(gameType string, raw json.RawMessage) (model.JSONB, error) {
	engine, err := f.registry.Get(gameType)
	if err != nil {
		return nil, err
	}

	return engine.NormalizeRoomSettings(raw)
}

func (f *Facade) ValidateRoomConfig(gameType string, maxPlayers int, settings model.JSONB) error {
	engine, err := f.registry.Get(gameType)
	if err != nil {
		return err
	}

	return engine.ValidateRoomConfig(maxPlayers, settings)
}

func (f *Facade) BuildInitialMatch(room *model.Room, participants []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error) {
	engine, err := f.registry.Get(room.GameType)
	if err != nil {
		return nil, nil, err
	}

	return engine.BuildInitialMatch(room, participants)
}

func (f *Facade) ApplyAction(ctx context.Context, match *model.Match, players []model.MatchPlayer, req ApplyActionRequest) (ApplyActionResult, error) {
	engine, err := f.registry.Get(match.GameType)
	if err != nil {
		return ApplyActionResult{}, err
	}

	return engine.ApplyAction(ctx, match, players, req)
}

func (f *Facade) ApplyTurnTimeout(ctx context.Context, match *model.Match, players []model.MatchPlayer) (ApplyActionResult, error) {
	engine, err := f.registry.Get(match.GameType)
	if err != nil {
		return ApplyActionResult{}, err
	}

	return engine.ApplyTurnTimeout(ctx, match, players)
}

func (f *Facade) BuildPublicState(match *model.Match, players []model.MatchPlayer) (json.RawMessage, error) {
	engine, err := f.registry.Get(match.GameType)
	if err != nil {
		return nil, err
	}

	return engine.BuildPublicState(match, players)
}

func (f *Facade) BuildPrivateState(match *model.Match, players []model.MatchPlayer, actorID string) (json.RawMessage, error) {
	engine, err := f.registry.Get(match.GameType)
	if err != nil {
		return nil, err
	}

	return engine.BuildPrivateState(match, players, actorID)
}
