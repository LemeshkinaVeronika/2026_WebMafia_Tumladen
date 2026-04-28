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

	var drawnTile *carcassonneDTO.TileView
	if state.CurrentTile != nil {
		tile, err := e.tileView(state.CurrentTile.TileID)
		if err != nil {
			return nil, err
		}
		drawnTile = tile
	} else if state.LastPlacedTile != nil {
		tile, err := e.tileView(state.LastPlacedTile.TileID)
		if err != nil {
			return nil, err
		}
		drawnTile = tile
	}

	currentPlayerID := currentPlayerIDView(state)
	result, err := publicMatchResult(match)
	if err != nil {
		return nil, err
	}

	publicState := carcassonneDTO.PublicGameState{
		Version:         state.Version,
		Status:          string(match.Status),
		Phase:           state.Phase,
		TurnNumber:      state.TurnNumber,
		CurrentPlayerID: currentPlayerID,
		Players:         state.Players,
		CurrentTurn: carcassonneDTO.CurrentTurnState{
			DrawnTile:    drawnTile,
			PlacedTile:   state.LastPlacedTile,
			MeeplePlaced: meeplePlacedOnLastTile(state),
		},
		Deck: carcassonneDTO.DeckState{
			RemainingCount: len(state.DeckRemaining),
		},
		Board: carcassonneDTO.BoardState{
			Tiles: state.Board,
		},
		Meeples:  e.publicMeeples(state),
		Settings: state.Settings,
		Result:   result,
	}

	data, err := json.Marshal(publicState)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *Engine) publicMeeples(state carcassonneDTO.GameState) []carcassonneDTO.PlacedMeeple {
	meeples := make([]carcassonneDTO.PlacedMeeple, 0, len(state.Meeples))
	for _, meeple := range state.Meeples {
		if meeple.FeatureType == "" {
			meeple.FeatureType = e.meepleFeatureType(state, meeple)
		}
		meeples = append(meeples, meeple)
	}
	return meeples
}

func (e *Engine) meepleFeatureType(state carcassonneDTO.GameState, meeple carcassonneDTO.PlacedMeeple) carcassonneDTO.ZoneType {
	tile := placedTileByInstanceID(state.Board, meeple.TileInstanceID)
	if tile == nil {
		return ""
	}
	def, ok := e.catalog.Get(tile.TileID)
	if !ok {
		return ""
	}
	zone, ok := findZone(def, meeple.ZoneID)
	if !ok {
		return ""
	}
	return zone.Type
}

func publicMatchResult(match *model.Match) (*carcassonneDTO.MatchResult, error) {
	if match == nil || match.Result == nil || len(*match.Result) == 0 || string(*match.Result) == "null" {
		return nil, nil
	}

	var result carcassonneDTO.MatchResult
	if err := json.Unmarshal(*match.Result, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (e *Engine) tileView(tileID string) (*carcassonneDTO.TileView, error) {
	def, ok := e.catalog.Get(tileID)
	if !ok {
		return nil, gameService.ErrInvalidMatchAction
	}

	return &carcassonneDTO.TileView{
		TileID:   def.TileID,
		ImageURL: def.ImageKey,
	}, nil
}

func currentPlayerIDView(state carcassonneDTO.GameState) *string {
	if state.Phase == carcassonneDTO.PhaseFinished || state.CurrentPlayerID == "" {
		return nil
	}

	currentPlayerID := state.CurrentPlayerID
	return &currentPlayerID
}

func meeplePlacedOnLastTile(state carcassonneDTO.GameState) bool {
	if state.LastPlacedTile == nil {
		return false
	}

	return slices.ContainsFunc(state.Meeples, func(meeple carcassonneDTO.PlacedMeeple) bool {
		return meeple.TileInstanceID == state.LastPlacedTile.InstanceID
	})
}

func (e *Engine) BuildPrivateState(match *model.Match, _ []model.MatchPlayer, actorID string) (json.RawMessage, error) {
	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return nil, err
	}

	privateState := carcassonneDTO.PrivateGameState{
		IsYourTurn:            state.CurrentPlayerID == actorID && state.Phase != carcassonneDTO.PhaseFinished,
		Phase:                 state.Phase,
		CurrentPlayerID:       currentPlayerIDView(state),
		AllowedActions:        []carcassonneDTO.Action{},
		ValidPlacements:       []carcassonneDTO.ValidTilePlacement{},
		ValidMeeplePlacements: []carcassonneDTO.ValidMeeplePlacement{},
	}

	if privateState.IsYourTurn {
		switch state.Phase {
		case carcassonneDTO.PhasePlaceTile:
			privateState.AllowedActions = []carcassonneDTO.Action{carcassonneDTO.ActionPlaceTile}
			privateState.ValidPlacements = e.validTilePlacements(state)
		case carcassonneDTO.PhasePlaceMeeple:
			privateState.ValidMeeplePlacements = e.validMeeplePlacements(state, actorID)
			if len(privateState.ValidMeeplePlacements) > 0 {
				privateState.AllowedActions = append(privateState.AllowedActions, carcassonneDTO.ActionPlaceMeeple)
			}
			privateState.AllowedActions = append(privateState.AllowedActions, carcassonneDTO.ActionSkipMeeple)
		}
	}

	data, err := json.Marshal(privateState)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *Engine) validTilePlacements(state carcassonneDTO.GameState) []carcassonneDTO.ValidTilePlacement {
	if state.CurrentTile == nil {
		return []carcassonneDTO.ValidTilePlacement{}
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

	placementsByCoordinate := make(map[[2]int][]int, len(candidates))
	for coordinate := range candidates {
		for _, rotation := range []int{0, 90, 180, 270} {
			if e.canPlaceTile(*state.CurrentTile, coordinate[0], coordinate[1], rotation, state.Board) {
				placementsByCoordinate[coordinate] = append(placementsByCoordinate[coordinate], rotation)
			}
		}
	}

	placements := make([]carcassonneDTO.ValidTilePlacement, 0, len(placementsByCoordinate))
	for coordinate, rotations := range placementsByCoordinate {
		placements = append(placements, carcassonneDTO.ValidTilePlacement{
			X:         coordinate[0],
			Y:         coordinate[1],
			Rotations: rotations,
		})
	}

	slices.SortFunc(placements, func(a, b carcassonneDTO.ValidTilePlacement) int {
		if a.X != b.X {
			return a.X - b.X
		}
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return 0
	})

	return placements
}

func (e *Engine) validMeeplePlacements(state carcassonneDTO.GameState, actorID string) []carcassonneDTO.ValidMeeplePlacement {
	if state.LastPlacedTile == nil || meeplesLeft(state.Players, actorID) <= 0 {
		return []carcassonneDTO.ValidMeeplePlacement{}
	}

	def, ok := e.catalog.Get(state.LastPlacedTile.TileID)
	if !ok {
		return []carcassonneDTO.ValidMeeplePlacement{}
	}

	placements := make([]carcassonneDTO.ValidMeeplePlacement, 0, len(def.Zones))
	for _, zone := range def.Zones {
		if e.canPlaceMeeple(state, actorID, zone.ZoneID) {
			segment := carcassonneDTO.ZoneSegment("")
			if len(zone.Segments) > 0 {
				segment = zone.Segments[0]
			}
			placements = append(placements, carcassonneDTO.ValidMeeplePlacement{
				ZoneID:      zone.ZoneID,
				FeatureType: zone.Type,
				Segment:     segment,
			})
		}
	}

	return placements
}
