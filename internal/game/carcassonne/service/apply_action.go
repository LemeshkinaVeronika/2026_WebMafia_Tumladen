package service

import (
	"context"
	"encoding/json"
	"slices"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) ApplyAction(ctx context.Context, match *model.Match, players []model.MatchPlayer, req gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	_ = ctx

	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	if state.Phase != carcassonneDTO.PhasePlaceTile && state.Phase != carcassonneDTO.PhasePlaceMeeple {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
	if state.CurrentPlayerID != req.ActorID {
		return gameService.ApplyActionResult{}, gameService.ErrNotYourTurn
	}
	if !slices.ContainsFunc(players, func(player model.MatchPlayer) bool {
		return player.ActorID == req.ActorID
	}) {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	switch req.Action {
	case string(carcassonneDTO.ActionPlaceTile):
		return e.placeTile(match, &state, req)
	case string(carcassonneDTO.ActionPlaceMeeple):
		return e.placeMeeple(match, &state, req)
	case string(carcassonneDTO.ActionSkipMeeple):
		return e.skipMeeple(match, &state, req)
	default:
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
}

func (e *Engine) placeTile(match *model.Match, state *carcassonneDTO.GameState, req gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	if state.Phase != carcassonneDTO.PhasePlaceTile || state.CurrentTile == nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	var payload carcassonneDTO.PlaceTilePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
	if payload.RoomID != "" && payload.RoomID != match.RoomID {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
	if !isValidRotation(payload.Rotation) {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
	if !e.canPlaceTile(*state.CurrentTile, payload.X, payload.Y, payload.Rotation, state.Board) {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	placedTile := carcassonneDTO.PlacedTile{
		InstanceID: state.CurrentTile.InstanceID,
		TileID:     state.CurrentTile.TileID,
		X:          payload.X,
		Y:          payload.Y,
		Rotation:   normalizeRotation(payload.Rotation),
		PlacedBy:   req.ActorID,
		TurnNumber: state.TurnNumber,
	}

	state.Board = append(state.Board, placedTile)
	state.LastPlacedTile = &placedTile
	state.CurrentTile = nil
	state.Phase = carcassonneDTO.PhasePlaceMeeple

	return marshalActionResult(*state, model.MatchStatusActive, nil)
}

func (e *Engine) placeMeeple(match *model.Match, state *carcassonneDTO.GameState, req gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	if state.Phase != carcassonneDTO.PhasePlaceMeeple || state.LastPlacedTile == nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	var payload carcassonneDTO.PlaceMeeplePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}
	if payload.RoomID != "" && payload.RoomID != match.RoomID {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	if !e.canPlaceMeeple(*state, req.ActorID, payload.ZoneID) {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	state.Meeples = append(state.Meeples, carcassonneDTO.PlacedMeeple{
		TileInstanceID: state.LastPlacedTile.InstanceID,
		ZoneID:         payload.ZoneID,
		ActorID:        req.ActorID,
	})
	for i := range state.Players {
		if state.Players[i].ActorID == req.ActorID {
			state.Players[i].MeeplesLeft--
			break
		}
	}

	return e.resolveTurn(state)
}

func (e *Engine) skipMeeple(match *model.Match, state *carcassonneDTO.GameState, req gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	if state.Phase != carcassonneDTO.PhasePlaceMeeple || state.LastPlacedTile == nil {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	var payload carcassonneDTO.SkipMeeplePayload
	if len(req.Payload) > 0 && string(req.Payload) != "null" {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
		}
	}
	if payload.RoomID != "" && payload.RoomID != match.RoomID {
		return gameService.ApplyActionResult{}, gameService.ErrInvalidMatchAction
	}

	return e.resolveTurn(state)
}

func (e *Engine) canPlaceTile(tile carcassonneDTO.TileInstance, x, y, rotation int, board []carcassonneDTO.PlacedTile) bool {
	if tileAt(board, x, y) != nil {
		return false
	}

	def, ok := e.catalog.Get(tile.TileID)
	if !ok {
		return false
	}
	edges := rotateEdges(def.Edges, rotation)
	hasNeighbor := false

	for _, neighbor := range adjacentTiles(board, x, y) {
		neighborDef, ok := e.catalog.Get(neighbor.tile.TileID)
		if !ok {
			return false
		}
		neighborEdges := rotateEdges(neighborDef.Edges, neighbor.tile.Rotation)
		hasNeighbor = true

		switch neighbor.direction {
		case directionTop:
			if edges.Top != neighborEdges.Bottom {
				return false
			}
		case directionRight:
			if edges.Right != neighborEdges.Left {
				return false
			}
		case directionBottom:
			if edges.Bottom != neighborEdges.Top {
				return false
			}
		case directionLeft:
			if edges.Left != neighborEdges.Right {
				return false
			}
		}
	}

	return hasNeighbor
}

func (e *Engine) canPlaceMeeple(state carcassonneDTO.GameState, actorID string, zoneID string) bool {
	if zoneID == "" || state.LastPlacedTile == nil {
		return false
	}
	if meeplesLeft(state.Players, actorID) <= 0 {
		return false
	}

	def, ok := e.catalog.Get(state.LastPlacedTile.TileID)
	if !ok {
		return false
	}

	zone, ok := findZone(def, zoneID)
	if !ok {
		return false
	}

	start := placedZoneRef{
		TileInstanceID: state.LastPlacedTile.InstanceID,
		ZoneID:         zoneID,
	}

	switch zone.Type {
	case carcassonneDTO.ZoneTypeMonastery:
		return !slices.ContainsFunc(state.Meeples, func(meeple carcassonneDTO.PlacedMeeple) bool {
			return meeple.TileInstanceID == state.LastPlacedTile.InstanceID && meeple.ZoneID == zoneID
		})

	case carcassonneDTO.ZoneTypeField:
		graph, err := e.buildFieldGraph(state)
		if err != nil {
			return false
		}
		root, ok := graph.dsu.find(start)
		if !ok {
			return false
		}
		return len(graph.meeplesByRoot[root]) == 0

	case carcassonneDTO.ZoneTypeRoad, carcassonneDTO.ZoneTypeCity:
		feature, err := e.buildFeature(state, start)
		if err != nil {
			return false
		}
		return len(feature.MeeplesByActor) == 0

	default:
		return false
	}
}

func marshalActionResult(state carcassonneDTO.GameState, status model.MatchStatus, result *model.JSONB) (gameService.ApplyActionResult, error) {
	rawState, err := json.Marshal(state)
	if err != nil {
		return gameService.ApplyActionResult{}, err
	}

	return gameService.ApplyActionResult{
		NextState:  model.JSONB(rawState),
		NextStatus: status,
		Result:     result,
	}, nil
}

func meeplesLeft(players []carcassonneDTO.PlayerState, actorID string) int {
	for _, player := range players {
		if player.ActorID == actorID {
			return player.MeeplesLeft
		}
	}
	return 0
}

func isValidRotation(rotation int) bool {
	normalized := normalizeRotation(rotation)
	return normalized == 0 || normalized == 90 || normalized == 180 || normalized == 270
}

func normalizeRotation(rotation int) int {
	rotation %= 360
	if rotation < 0 {
		rotation += 360
	}
	return rotation
}

func rotateEdges(edges carcassonneDTO.TileEdges, rotation int) carcassonneDTO.TileEdges {
	for i := 0; i < normalizeRotation(rotation)/90; i++ {
		edges = carcassonneDTO.TileEdges{
			Top:    edges.Left,
			Right:  edges.Top,
			Bottom: edges.Right,
			Left:   edges.Bottom,
		}
	}
	return edges
}

type direction int

const (
	directionTop direction = iota
	directionRight
	directionBottom
	directionLeft
)

type adjacentTile struct {
	tile      carcassonneDTO.PlacedTile
	direction direction
}

func adjacentTiles(board []carcassonneDTO.PlacedTile, x, y int) []adjacentTile {
	neighbors := make([]adjacentTile, 0, 4)
	if tile := tileAt(board, x, y-1); tile != nil {
		neighbors = append(neighbors, adjacentTile{tile: *tile, direction: directionTop})
	}
	if tile := tileAt(board, x+1, y); tile != nil {
		neighbors = append(neighbors, adjacentTile{tile: *tile, direction: directionRight})
	}
	if tile := tileAt(board, x, y+1); tile != nil {
		neighbors = append(neighbors, adjacentTile{tile: *tile, direction: directionBottom})
	}
	if tile := tileAt(board, x-1, y); tile != nil {
		neighbors = append(neighbors, adjacentTile{tile: *tile, direction: directionLeft})
	}
	return neighbors
}

func tileAt(board []carcassonneDTO.PlacedTile, x, y int) *carcassonneDTO.PlacedTile {
	for i := range board {
		if board[i].X == x && board[i].Y == y {
			return &board[i]
		}
	}
	return nil
}
