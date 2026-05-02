package service

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

func TestApplyActionRunsFullTurnContract(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceTile,
		TurnNumber:      1,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", DisplayName: "A", MeeplesLeft: 7},
			{ActorID: "actor-b", DisplayName: "B", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
		},
		DeckRemaining: []carcassonneDTO.TileInstance{
			{InstanceID: "next-tile", TileID: "road_straight"},
		},
		CurrentTile: &carcassonneDTO.TileInstance{InstanceID: "drawn-tile", TileID: "city_cap"},
		Meeples:     []carcassonneDTO.PlacedMeeple{},
	}
	match := matchFromGameState(t, state)
	players := []model.MatchPlayer{
		{MatchID: match.ID, ActorID: "actor-a"},
		{MatchID: match.ID, ActorID: "actor-b"},
	}

	privateBefore := privateStateForActor(t, engine, match, players, "actor-a")
	if !privateBefore.IsYourTurn || len(privateBefore.ValidPlacements) == 0 {
		t.Fatalf("current actor private state = %#v, want tile placement hints", privateBefore)
	}
	placement := privateBefore.ValidPlacements[0]
	rotation := placement.Rotations[0]

	placePayload := marshalPayload(t, carcassonneDTO.PlaceTilePayload{
		RoomID:   match.RoomID,
		X:        placement.X,
		Y:        placement.Y,
		Rotation: rotation,
	})
	placeResult, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-a",
		Action:  string(carcassonneDTO.ActionPlaceTile),
		Payload: placePayload,
	})
	if err != nil {
		t.Fatalf("place tile: %v", err)
	}

	var afterPlace carcassonneDTO.GameState
	unmarshalGameState(t, placeResult.NextState, &afterPlace)
	if afterPlace.Phase != carcassonneDTO.PhasePlaceMeeple {
		t.Fatalf("phase after place tile = %s, want %s", afterPlace.Phase, carcassonneDTO.PhasePlaceMeeple)
	}
	if afterPlace.CurrentTile != nil {
		t.Fatalf("currentTile after place tile = %#v, want nil", afterPlace.CurrentTile)
	}
	if afterPlace.LastPlacedTile == nil || afterPlace.LastPlacedTile.InstanceID != "drawn-tile" {
		t.Fatalf("lastPlacedTile after place tile = %#v, want drawn-tile", afterPlace.LastPlacedTile)
	}
	if got, want := len(afterPlace.Board), 2; got != want {
		t.Fatalf("board size after place tile = %d, want %d", got, want)
	}

	match.GameState = placeResult.NextState
	privateAfterPlace := privateStateForActor(t, engine, match, players, "actor-a")
	if len(privateAfterPlace.AllowedActions) == 0 || privateAfterPlace.AllowedActions[len(privateAfterPlace.AllowedActions)-1] != carcassonneDTO.ActionSkipMeeple {
		t.Fatalf("allowed actions after place tile = %#v, want skip_meeple allowed", privateAfterPlace.AllowedActions)
	}
	otherPrivate := privateStateForActor(t, engine, match, players, "actor-b")
	if otherPrivate.IsYourTurn || len(otherPrivate.AllowedActions) != 0 || len(otherPrivate.ValidMeeplePlacements) != 0 {
		t.Fatalf("other actor private state leaked hints: %#v", otherPrivate)
	}

	publicRaw, err := engine.BuildPublicState(match, players)
	if err != nil {
		t.Fatalf("BuildPublicState() after place tile error = %v", err)
	}
	if strings.Contains(string(publicRaw), "deckRemaining") || strings.Contains(string(publicRaw), "next-tile") {
		t.Fatalf("public state leaked server deck: %s", publicRaw)
	}

	skipPayload := marshalPayload(t, carcassonneDTO.SkipMeeplePayload{RoomID: match.RoomID})
	skipResult, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-a",
		Action:  string(carcassonneDTO.ActionSkipMeeple),
		Payload: skipPayload,
	})
	if err != nil {
		t.Fatalf("skip meeple: %v", err)
	}

	var afterSkip carcassonneDTO.GameState
	unmarshalGameState(t, skipResult.NextState, &afterSkip)
	if afterSkip.Phase != carcassonneDTO.PhasePlaceTile {
		t.Fatalf("phase after skip meeple = %s, want %s", afterSkip.Phase, carcassonneDTO.PhasePlaceTile)
	}
	if afterSkip.CurrentPlayerID != "actor-b" {
		t.Fatalf("currentPlayerID after skip meeple = %s, want actor-b", afterSkip.CurrentPlayerID)
	}
	if afterSkip.CurrentTile == nil || afterSkip.CurrentTile.InstanceID != "next-tile" {
		t.Fatalf("currentTile after skip meeple = %#v, want next-tile", afterSkip.CurrentTile)
	}
	if afterSkip.LastPlacedTile != nil {
		t.Fatalf("lastPlacedTile after turn resolve = %#v, want nil", afterSkip.LastPlacedTile)
	}
	if got, want := len(afterSkip.DeckRemaining), 0; got != want {
		t.Fatalf("deck size after drawing next tile = %d, want %d", got, want)
	}
}

func TestApplyActionRejectsInvalidTurnAndPlacements(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceTile,
		TurnNumber:      1,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
		},
		CurrentTile: &carcassonneDTO.TileInstance{InstanceID: "drawn-tile", TileID: "city_cap"},
	}
	match := matchFromGameState(t, state)
	players := []model.MatchPlayer{
		{MatchID: match.ID, ActorID: "actor-a"},
		{MatchID: match.ID, ActorID: "actor-b"},
	}

	if _, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-b",
		Action:  string(carcassonneDTO.ActionPlaceTile),
		Payload: marshalPayload(t, carcassonneDTO.PlaceTilePayload{RoomID: match.RoomID, X: 1, Y: 0, Rotation: 0}),
	}); err != gameService.ErrNotYourTurn {
		t.Fatalf("wrong actor error = %v, want ErrNotYourTurn", err)
	}

	if _, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-a",
		Action:  string(carcassonneDTO.ActionPlaceTile),
		Payload: marshalPayload(t, carcassonneDTO.PlaceTilePayload{RoomID: match.RoomID, X: 0, Y: 0, Rotation: 0}),
	}); err != gameService.ErrInvalidMatchAction {
		t.Fatalf("occupied tile placement error = %v, want ErrInvalidMatchAction", err)
	}

	if _, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-a",
		Action:  string(carcassonneDTO.ActionPlaceMeeple),
		Payload: marshalPayload(t, carcassonneDTO.PlaceMeeplePayload{RoomID: match.RoomID, ZoneID: "city_1"}),
	}); err != gameService.ErrInvalidMatchAction {
		t.Fatalf("meeple in tile phase error = %v, want ErrInvalidMatchAction", err)
	}
}

func TestApplyActionStoresMeepleFeatureType(t *testing.T) {
	engine := testEngine(t)

	lastPlacedTile := carcassonneDTO.PlacedTile{
		InstanceID: "drawn-tile",
		TileID:     "road_straight",
		X:          0,
		Y:          -1,
		Rotation:   0,
		PlacedBy:   "actor-a",
		TurnNumber: 1,
	}
	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceMeeple,
		TurnNumber:      1,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			lastPlacedTile,
		},
		LastPlacedTile: &lastPlacedTile,
		DeckRemaining: []carcassonneDTO.TileInstance{
			{InstanceID: "next-tile", TileID: "city_cap"},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{},
	}
	match := matchFromGameState(t, state)
	players := []model.MatchPlayer{
		{MatchID: match.ID, ActorID: "actor-a"},
		{MatchID: match.ID, ActorID: "actor-b"},
	}

	result, err := engine.ApplyAction(t.Context(), match, players, gameService.ApplyActionRequest{
		ActorID: "actor-a",
		Action:  string(carcassonneDTO.ActionPlaceMeeple),
		Payload: marshalPayload(t, carcassonneDTO.PlaceMeeplePayload{RoomID: match.RoomID, ZoneID: "road_1"}),
	})
	if err != nil {
		t.Fatalf("place meeple: %v", err)
	}

	var after carcassonneDTO.GameState
	unmarshalGameState(t, result.NextState, &after)
	if got, want := len(after.Meeples), 1; got != want {
		t.Fatalf("meeples after place = %d, want %d", got, want)
	}
	if got, want := after.Meeples[0].FeatureType, carcassonneDTO.ZoneTypeRoad; got != want {
		t.Fatalf("meeple featureType = %s, want %s", got, want)
	}
}

func TestApplyTurnTimeoutPlacesFirstValidTileAndSkipsMeeple(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceTile,
		TurnNumber:      1,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
		},
		DeckRemaining: []carcassonneDTO.TileInstance{
			{InstanceID: "next-tile", TileID: "road_straight"},
		},
		CurrentTile: &carcassonneDTO.TileInstance{InstanceID: "drawn-tile", TileID: "city_cap"},
		Meeples:     []carcassonneDTO.PlacedMeeple{},
	}
	match := matchFromGameState(t, state)
	players := []model.MatchPlayer{
		{MatchID: match.ID, ActorID: "actor-a"},
		{MatchID: match.ID, ActorID: "actor-b"},
	}

	privateBefore := privateStateForActor(t, engine, match, players, "actor-a")
	validPlacements := privateBefore.ValidPlacements

	result, err := engine.ApplyTurnTimeout(t.Context(), match, players)
	if err != nil {
		t.Fatalf("ApplyTurnTimeout() error = %v", err)
	}

	var after carcassonneDTO.GameState
	unmarshalGameState(t, result.NextState, &after)
	if after.Phase != carcassonneDTO.PhasePlaceTile {
		t.Fatalf("phase after timeout = %s, want %s", after.Phase, carcassonneDTO.PhasePlaceTile)
	}
	if after.CurrentPlayerID != "actor-b" {
		t.Fatalf("currentPlayerID after timeout = %s, want actor-b", after.CurrentPlayerID)
	}
	if after.CurrentTile == nil || after.CurrentTile.InstanceID != "next-tile" {
		t.Fatalf("currentTile after timeout = %#v, want next-tile", after.CurrentTile)
	}
	if after.LastPlacedTile != nil {
		t.Fatalf("lastPlacedTile after timeout = %#v, want nil", after.LastPlacedTile)
	}
	placed := placedTileByInstanceID(after.Board, "drawn-tile")
	if placed == nil {
		t.Fatalf("placed tile after timeout = nil, want drawn-tile")
	}
	if !containsValidPlacement(validPlacements, placed.X, placed.Y, placed.Rotation) {
		t.Fatalf("placed tile = %#v, want one of %#v", placed, validPlacements)
	}
	if len(after.Meeples) != 0 {
		t.Fatalf("meeples after timeout = %d, want 0", len(after.Meeples))
	}
}

func TestApplyTurnTimeoutSkipsMeeple(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "drawn-tile",
		TileID:     "city_cap",
		X:          1,
		Y:          0,
		Rotation:   0,
		PlacedBy:   "actor-a",
		TurnNumber: 1,
	}
	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceMeeple,
		TurnNumber:      1,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
			lastPlaced,
		},
		DeckRemaining: []carcassonneDTO.TileInstance{
			{InstanceID: "next-tile", TileID: "road_straight"},
		},
		LastPlacedTile: &lastPlaced,
		Meeples:        []carcassonneDTO.PlacedMeeple{},
	}
	match := matchFromGameState(t, state)
	players := []model.MatchPlayer{
		{MatchID: match.ID, ActorID: "actor-a"},
		{MatchID: match.ID, ActorID: "actor-b"},
	}

	result, err := engine.ApplyTurnTimeout(t.Context(), match, players)
	if err != nil {
		t.Fatalf("ApplyTurnTimeout() error = %v", err)
	}

	var after carcassonneDTO.GameState
	unmarshalGameState(t, result.NextState, &after)
	if after.Phase != carcassonneDTO.PhasePlaceTile {
		t.Fatalf("phase after timeout = %s, want %s", after.Phase, carcassonneDTO.PhasePlaceTile)
	}
	if after.CurrentPlayerID != "actor-b" {
		t.Fatalf("currentPlayerID after timeout = %s, want actor-b", after.CurrentPlayerID)
	}
	if len(after.Meeples) != 0 {
		t.Fatalf("meeples after timeout = %d, want 0", len(after.Meeples))
	}
}

func matchFromGameState(t *testing.T, state carcassonneDTO.GameState) *model.Match {
	t.Helper()

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	return &model.Match{
		ID:        "match-1",
		RoomID:    "room-1",
		GameType:  "carcassonne",
		Status:    model.MatchStatusActive,
		GameState: model.JSONB(raw),
	}
}

func marshalPayload(t *testing.T, payload any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return raw
}

func privateStateForActor(
	t *testing.T,
	engine *Engine,
	match *model.Match,
	players []model.MatchPlayer,
	actorID string,
) carcassonneDTO.PrivateGameState {
	t.Helper()

	raw, err := engine.BuildPrivateState(match, players, actorID)
	if err != nil {
		t.Fatalf("BuildPrivateState(%s): %v", actorID, err)
	}

	var state carcassonneDTO.PrivateGameState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("unmarshal private state: %v", err)
	}
	return state
}

func unmarshalGameState(t *testing.T, raw model.JSONB, state *carcassonneDTO.GameState) {
	t.Helper()

	if err := json.Unmarshal(raw, state); err != nil {
		t.Fatalf("unmarshal game state: %v", err)
	}
}

func containsValidPlacement(placements []carcassonneDTO.ValidTilePlacement, x, y, rotation int) bool {
	for _, placement := range placements {
		if placement.X != x || placement.Y != y {
			continue
		}
		return slices.Contains(placement.Rotations, rotation)
	}
	return false
}
