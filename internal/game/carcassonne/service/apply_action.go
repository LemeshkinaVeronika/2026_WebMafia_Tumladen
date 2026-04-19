package service

import (
	"context"
	"encoding/json"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) ApplyAction(ctx context.Context, match *model.Match, players []model.MatchPlayer, req gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	_ = ctx
	_ = players

	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	switch req.Action {
	case "advance_turn":
		nextState, err := e.applyAdvanceTurn(state, req.ActorID)
		if err != nil {
			return gameService.ApplyActionResult{}, err
		}

		return gameService.ApplyActionResult{
			NextState:  nextState,
			NextStatus: model.MatchStatusActive,
			Result:     nil,
		}, nil
	default:
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
}

func (e *Engine) applyAdvanceTurn(state carcassonneDTO.GameState, actorID string) (model.JSONB, error) {
	if len(state.Players) == 0 {
		return nil, gameService.ErrInvalidMatchAction
	}

	if state.CurrentPlayerID != actorID {
		return nil, gameService.ErrNotYourTurn
	}

	currentIndex := -1
	for i, player := range state.Players {
		if player.ActorID == state.CurrentPlayerID {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		return nil, gameService.ErrInvalidMatchAction
	}

	nextIndex := (currentIndex + 1) % len(state.Players)
	state.CurrentPlayerID = state.Players[nextIndex].ActorID
	state.TurnNumber++

	normalized, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	return model.JSONB(normalized), nil
}
