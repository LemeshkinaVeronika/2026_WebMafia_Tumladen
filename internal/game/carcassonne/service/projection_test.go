package service

import (
	"encoding/json"
	"testing"
	"time"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	"github.com/webmafia/tumladan/internal/model"
)

func TestBuildPublicStateExposesExplicitTurnDeckAndBoard(t *testing.T) {
	engine := testEngine(t)
	updatedAt := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceMeeple,
		TurnNumber:      3,
		TurnStartedAt:   "2026-04-30T09:59:00Z",
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", DisplayName: "A", Score: 4, MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
			{InstanceID: "placed", TileID: "city_cap", X: 0, Y: -1, Rotation: 180, PlacedBy: "actor-a", TurnNumber: 3},
		},
		LastPlacedTile: &carcassonneDTO.PlacedTile{
			InstanceID: "placed",
			TileID:     "city_cap",
			X:          0,
			Y:          -1,
			Rotation:   180,
			PlacedBy:   "actor-a",
			TurnNumber: 3,
		},
		DeckRemaining: []carcassonneDTO.TileInstance{
			{InstanceID: "next", TileID: "monastery"},
		},
		Settings: carcassonneDTO.MatchSettings{
			TurnTimeSeconds: 120,
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "placed", ZoneID: "city_1", ActorID: "actor-a"},
		},
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	rawPublic, err := engine.BuildPublicState(&model.Match{GameState: model.JSONB(rawState), UpdatedAt: updatedAt}, nil)
	if err != nil {
		t.Fatalf("BuildPublicState() error = %v", err)
	}

	var publicState carcassonneDTO.PublicGameState
	if err := json.Unmarshal(rawPublic, &publicState); err != nil {
		t.Fatalf("unmarshal public state: %v", err)
	}

	if publicState.CurrentPlayerID == nil || *publicState.CurrentPlayerID != "actor-a" {
		t.Fatalf("currentPlayerId = %v, want actor-a", publicState.CurrentPlayerID)
	}
	if publicState.CurrentTurn.DrawnTile == nil || publicState.CurrentTurn.DrawnTile.ImageURL != "city_cap.webp" {
		t.Fatalf("drawnTile = %#v, want city_cap.webp", publicState.CurrentTurn.DrawnTile)
	}
	if publicState.CurrentTurn.PlacedTile == nil || publicState.CurrentTurn.PlacedTile.InstanceID != "placed" {
		t.Fatalf("placedTile = %#v, want placed", publicState.CurrentTurn.PlacedTile)
	}
	if !publicState.CurrentTurn.MeeplePlaced {
		t.Fatal("meeplePlaced = false, want true")
	}
	if publicState.CurrentTurn.TurnEndsAt == nil || *publicState.CurrentTurn.TurnEndsAt != "2026-04-30T10:01:00Z" {
		t.Fatalf("turnEndsAt = %v, want 2026-04-30T10:01:00Z", publicState.CurrentTurn.TurnEndsAt)
	}
	if got, want := publicState.Deck.RemainingCount, 1; got != want {
		t.Fatalf("remainingCount = %d, want %d", got, want)
	}
	if got, want := len(publicState.Board.Tiles), 2; got != want {
		t.Fatalf("board tiles = %d, want %d", got, want)
	}
	if got, want := publicState.Meeples[0].FeatureType, carcassonneDTO.ZoneTypeCity; got != want {
		t.Fatalf("meeple featureType = %s, want %s", got, want)
	}
	if got, want := publicState.Meeples[0].Segment, carcassonneDTO.SegmentBottomCenter; got != want {
		t.Fatalf("meeple segment = %s, want %s", got, want)
	}
}

func TestBuildPrivateStateExposesActionsAndGroupedPlacements(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceTile,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
		},
		CurrentTile: &carcassonneDTO.TileInstance{InstanceID: "tile-1", TileID: "city_cap"},
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	match := &model.Match{GameState: model.JSONB(rawState)}

	rawPrivate, err := engine.BuildPrivateState(match, nil, "actor-a")
	if err != nil {
		t.Fatalf("BuildPrivateState() error = %v", err)
	}

	var privateState carcassonneDTO.PrivateGameState
	if err := json.Unmarshal(rawPrivate, &privateState); err != nil {
		t.Fatalf("unmarshal private state: %v", err)
	}

	if !privateState.IsYourTurn {
		t.Fatal("isYourTurn = false, want true")
	}
	if len(privateState.AllowedActions) != 1 || privateState.AllowedActions[0] != carcassonneDTO.ActionPlaceTile {
		t.Fatalf("allowedActions = %#v, want place_tile", privateState.AllowedActions)
	}
	if len(privateState.ValidPlacements) == 0 {
		t.Fatal("validPlacements is empty")
	}
	for _, placement := range privateState.ValidPlacements {
		if len(placement.Rotations) == 0 {
			t.Fatalf("placement %#v has no rotations", placement)
		}
	}

	rawPrivate, err = engine.BuildPrivateState(match, nil, "actor-b")
	if err != nil {
		t.Fatalf("BuildPrivateState() for other actor error = %v", err)
	}
	if err := json.Unmarshal(rawPrivate, &privateState); err != nil {
		t.Fatalf("unmarshal other private state: %v", err)
	}
	if privateState.IsYourTurn {
		t.Fatal("other actor isYourTurn = true, want false")
	}
	if len(privateState.AllowedActions) != 0 || len(privateState.ValidPlacements) != 0 || len(privateState.ValidMeeplePlacements) != 0 {
		t.Fatalf("other actor private hints are not empty: %#v", privateState)
	}
}

func TestBuildPrivateStateExposesMeepleActionsAndZones(t *testing.T) {
	engine := testEngine(t)

	lastPlacedTile := carcassonneDTO.PlacedTile{
		InstanceID: "tile-1",
		TileID:     "city_cap",
		X:          0,
		Y:          -1,
		Rotation:   180,
		PlacedBy:   "actor-a",
		TurnNumber: 1,
	}
	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceMeeple,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
			lastPlacedTile,
		},
		LastPlacedTile: &lastPlacedTile,
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	rawPrivate, err := engine.BuildPrivateState(&model.Match{GameState: model.JSONB(rawState)}, nil, "actor-a")
	if err != nil {
		t.Fatalf("BuildPrivateState() error = %v", err)
	}

	var privateState carcassonneDTO.PrivateGameState
	if err := json.Unmarshal(rawPrivate, &privateState); err != nil {
		t.Fatalf("unmarshal private state: %v", err)
	}

	if len(privateState.AllowedActions) != 2 ||
		privateState.AllowedActions[0] != carcassonneDTO.ActionPlaceMeeple ||
		privateState.AllowedActions[1] != carcassonneDTO.ActionSkipMeeple {
		t.Fatalf("allowedActions = %#v, want place_meeple and skip_meeple", privateState.AllowedActions)
	}
	if len(privateState.ValidMeeplePlacements) == 0 {
		t.Fatal("validMeeplePlacements is empty")
	}
	for _, placement := range privateState.ValidMeeplePlacements {
		if placement.TileInstanceID != lastPlacedTile.InstanceID || placement.X != lastPlacedTile.X || placement.Y != lastPlacedTile.Y {
			t.Fatalf("meeple placement is not anchored to last tile: %#v", placement)
		}
		if placement.ZoneID == "" || placement.FeatureType == "" || placement.Segment == "" {
			t.Fatalf("invalid meeple placement hint: %#v", placement)
		}
	}
}

func TestBuildPrivateStateRotatesMeeplePlacementSegments(t *testing.T) {
	engine := testEngine(t)

	lastPlacedTile := carcassonneDTO.PlacedTile{
		InstanceID: "tile-1",
		TileID:     "city_curve_with_road_curve_shield",
		X:          0,
		Y:          0,
		Rotation:   180,
		PlacedBy:   "actor-a",
		TurnNumber: 1,
	}
	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhasePlaceMeeple,
		CurrentPlayerID: "actor-a",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 7},
		},
		Board:          []carcassonneDTO.PlacedTile{lastPlacedTile},
		LastPlacedTile: &lastPlacedTile,
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	rawPrivate, err := engine.BuildPrivateState(&model.Match{GameState: model.JSONB(rawState)}, nil, "actor-a")
	if err != nil {
		t.Fatalf("BuildPrivateState() error = %v", err)
	}

	var privateState carcassonneDTO.PrivateGameState
	if err := json.Unmarshal(rawPrivate, &privateState); err != nil {
		t.Fatalf("unmarshal private state: %v", err)
	}

	for _, placement := range privateState.ValidMeeplePlacements {
		if placement.ZoneID == "field_2" {
			if placement.TileInstanceID != lastPlacedTile.InstanceID || placement.X != lastPlacedTile.X || placement.Y != lastPlacedTile.Y {
				t.Fatalf("field_2 placement anchor = %#v, want last placed tile", placement)
			}
			if got, want := placement.Segment, carcassonneDTO.SegmentTopRight; got != want {
				t.Fatalf("field_2 segment = %s, want rotated %s", got, want)
			}
			return
		}
	}
	t.Fatal("field_2 meeple placement not found")
}

func TestBuildPublicStateBackfillsRotatedMeepleSegment(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version: 1,
		Phase:   carcassonneDTO.PhasePlaceTile,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "tile-1", TileID: "city_curve_with_road_curve_shield", X: 0, Y: 0, Rotation: 180},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "tile-1", ZoneID: "field_2", ActorID: "actor-a"},
		},
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	rawPublic, err := engine.BuildPublicState(&model.Match{GameState: model.JSONB(rawState)}, nil)
	if err != nil {
		t.Fatalf("BuildPublicState() error = %v", err)
	}

	var publicState carcassonneDTO.PublicGameState
	if err := json.Unmarshal(rawPublic, &publicState); err != nil {
		t.Fatalf("unmarshal public state: %v", err)
	}
	if got, want := publicState.Meeples[0].FeatureType, carcassonneDTO.ZoneTypeField; got != want {
		t.Fatalf("meeple featureType = %s, want %s", got, want)
	}
	if got, want := publicState.Meeples[0].Segment, carcassonneDTO.SegmentTopRight; got != want {
		t.Fatalf("meeple segment = %s, want rotated %s", got, want)
	}
}

func TestMatchResultIncludesWinnersAndFinalScores(t *testing.T) {
	result := matchResult([]carcassonneDTO.PlayerState{
		{ActorID: "actor-a", Score: 10},
		{ActorID: "actor-b", Score: 12},
		{ActorID: "actor-c", Score: 12},
	})

	if got, want := len(result.Winners), 2; got != want {
		t.Fatalf("winners count = %d, want %d", got, want)
	}
	if result.Winners[0] != "actor-b" || result.Winners[1] != "actor-c" {
		t.Fatalf("winners = %#v, want actor-b actor-c", result.Winners)
	}
	if got, want := len(result.FinalScores), 3; got != want {
		t.Fatalf("finalScores count = %d, want %d", got, want)
	}
}

func TestBuildPublicStateFinishedHasNoCurrentPlayer(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhaseFinished,
		CurrentPlayerID: "",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", Score: 10},
		},
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	rawPublic, err := engine.BuildPublicState(&model.Match{
		GameState: model.JSONB(rawState),
	}, nil)
	if err != nil {
		t.Fatalf("BuildPublicState() error = %v", err)
	}

	var publicState carcassonneDTO.PublicGameState
	if err := json.Unmarshal(rawPublic, &publicState); err != nil {
		t.Fatalf("unmarshal public state: %v", err)
	}

	if publicState.CurrentPlayerID != nil {
		t.Fatalf("currentPlayerId = %v, want nil", *publicState.CurrentPlayerID)
	}
}

func TestBuildPublicStateFinishedExposesResult(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Version:         1,
		Phase:           carcassonneDTO.PhaseFinished,
		CurrentPlayerID: "",
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", Score: 10},
			{ActorID: "actor-b", Score: 12},
		},
	}
	rawState, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	result, err := json.Marshal(matchResult(state.Players))
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	rawPublic, err := engine.BuildPublicState(&model.Match{
		GameState: model.JSONB(rawState),
		Result:    jsonb(result),
	}, nil)
	if err != nil {
		t.Fatalf("BuildPublicState() error = %v", err)
	}

	var publicState carcassonneDTO.PublicGameState
	if err := json.Unmarshal(rawPublic, &publicState); err != nil {
		t.Fatalf("unmarshal public state: %v", err)
	}

	if publicState.Result == nil {
		t.Fatal("result = nil, want match result")
	}
	if len(publicState.Result.Winners) != 1 || publicState.Result.Winners[0] != "actor-b" {
		t.Fatalf("winners = %#v, want actor-b", publicState.Result.Winners)
	}
}

func jsonb(raw []byte) *model.JSONB {
	value := model.JSONB(raw)
	return &value
}
