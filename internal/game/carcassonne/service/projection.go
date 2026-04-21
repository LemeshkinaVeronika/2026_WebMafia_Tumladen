package service

import (
	"encoding/json"
	"slices"

	gameService "github.com/webmafia/tumladan/internal/game/service"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) BuildPublicState(match *model.Match, _ []model.MatchPlayer) (json.RawMessage, error) {
	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return nil, err
	}

	var currentTile *carcassonneDTO.TileView
	if state.CurrentTile != nil {
		def, ok := e.catalog.Get(state.CurrentTile.TileID)
		if !ok {
			return nil, gameService.ErrInvalidMatchAction
		}

		currentTile = &carcassonneDTO.TileView{
			TileID:   def.TileID,
			ImageKey: def.ImageKey,
		}
	}

	publicState := carcassonneDTO.PublicGameState{
		Version:         state.Version,
		Phase:           state.Phase,
		TurnNumber:      state.TurnNumber,
		CurrentPlayerID: state.CurrentPlayerID,
		Players:         state.Players,
		Board:           state.Board,
		CurrentTile:     currentTile,
		DeckRemaining:   len(state.DeckRemaining),
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

	privateState := carcassonneDTO.PrivateGameState{}

	if state.CurrentPlayerID == actorID {
		switch state.Phase {
		case carcassonneDTO.PhasePlaceTile:
			privateState.AllowedTilePlacements = e.allowedTilePlacements(state)
		case carcassonneDTO.PhasePlaceMeeple:
			privateState.AllowedMeepleZones = e.allowedMeepleZones(state, actorID)
			privateState.CanSkipMeeple = true
		}
	}

	data, err := json.Marshal(privateState)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *Engine) allowedTilePlacements(state carcassonneDTO.GameState) []carcassonneDTO.AllowedTilePlacement {
	if state.CurrentTile == nil {
		return []carcassonneDTO.AllowedTilePlacement{}
	}

	candidates := make(map[[2]int]struct{}, len(state.Board)*4)
	for _, tile := range state.Board {
		for _, coordinate := range [][2]int{
			{tile.X, tile.Y - 1},
			{tile.X + 1, tile.Y},
			{tile.X, tile.Y + 1},
			{tile.X - 1, tile.Y},
		} {
			if tileAt(state.Board, coordinate[0], coordinate[1]) == nil {
				candidates[coordinate] = struct{}{}
			}
		}
	}

	placements := make([]carcassonneDTO.AllowedTilePlacement, 0, len(candidates)*4)
	for coordinate := range candidates {
		for _, rotation := range []int{0, 90, 180, 270} {
			if e.canPlaceTile(*state.CurrentTile, coordinate[0], coordinate[1], rotation, state.Board) {
				placements = append(placements, carcassonneDTO.AllowedTilePlacement{
					X:        coordinate[0],
					Y:        coordinate[1],
					Rotation: rotation,
				})
			}
		}
	}

	slices.SortFunc(placements, func(a, b carcassonneDTO.AllowedTilePlacement) int {
		if a.X != b.X {
			return a.X - b.X
		}
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.Rotation - b.Rotation
	})

	return placements
}

func (e *Engine) allowedMeepleZones(state carcassonneDTO.GameState, actorID string) []string {
	if state.LastPlacedTile == nil || meeplesLeft(state.Players, actorID) <= 0 {
		return []string{}
	}

	def, ok := e.catalog.Get(state.LastPlacedTile.TileID)
	if !ok {
		return []string{}
	}

	zones := make([]string, 0, len(def.Zones))
	for _, zone := range def.Zones {
		if e.canPlaceMeeple(state, actorID, zone.ZoneID) {
			zones = append(zones, zone.ZoneID)
		}
	}

	return zones
}
