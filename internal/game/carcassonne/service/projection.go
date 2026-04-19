package service

import (
	"encoding/json"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) BuildPublicState(match *model.Match, _ []model.MatchPlayer) (json.RawMessage, error) {
	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return nil, err
	}

	publicState := carcassonneDTO.PublicGameState{
		Version:         state.Version,
		Phase:           state.Phase,
		TurnNumber:      state.TurnNumber,
		CurrentPlayerID: state.CurrentPlayerID,
		Players:         state.Players,
		Board:           state.Board,
		CurrentTile:     state.CurrentTile,
		DeckRemaining:   state.DeckRemaining,
		Settings:        state.Settings,
	}

	data, err := json.Marshal(publicState)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *Engine) BuildPrivateState(match *model.Match, _ []model.MatchPlayer, actorID string) (json.RawMessage, error) {
	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return nil, err
	}

	payload := struct {
		IsYourTurn bool `json:"isYourTurn"`
	}{
		IsYourTurn: state.CurrentPlayerID != "" && state.CurrentPlayerID == actorID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return data, nil
}
