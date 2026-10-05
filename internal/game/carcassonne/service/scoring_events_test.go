package service

import (
	"encoding/json"
	"testing"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
)

func TestCompletedRoadEmitsPerTileContributionsAndTieAwards(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "road-end-bottom",
		TileID:     "monastery_road",
		X:          0,
		Y:          1,
		Rotation:   180,
		TurnNumber: 7,
	}
	state := carcassonneDTO.GameState{
		TurnNumber: 7,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-b", MeeplesLeft: 6},
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "road-end-top", TileID: "monastery_road", X: 0, Y: -1, Rotation: 0},
			{InstanceID: "road-middle", TileID: "road_straight", X: 0, Y: 0, Rotation: 0},
			lastPlaced,
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "road-end-top", ZoneID: "road_1", ActorID: "actor-b"},
			{TileInstanceID: "road-middle", ZoneID: "road_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreCompletedRoadsWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreCompletedRoadsWithEvents() error = %v", err)
	}

	if got, want := len(events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	payload := featureScoredPayload(t, events[0])
	if got, want := payload.ScoringPhase, carcassonneDTO.ScoringPhaseTurn; got != want {
		t.Fatalf("scoring phase = %s, want %s", got, want)
	}
	if got, want := payload.FeatureType, carcassonneDTO.ZoneTypeRoad; got != want {
		t.Fatalf("feature type = %s, want %s", got, want)
	}
	if got, want := payload.AnchorTileInstanceID, lastPlaced.InstanceID; got != want {
		t.Fatalf("anchor tile = %s, want %s", got, want)
	}
	if got, want := payload.TotalPoints, 3; got != want {
		t.Fatalf("total points = %d, want %d", got, want)
	}
	if got, want := scoreContributionTotal(payload.Contributions), payload.TotalPoints; got != want {
		t.Fatalf("contribution total = %d, want %d", got, want)
	}
	if got, want := len(payload.Contributions), 3; got != want {
		t.Fatalf("contribution count = %d, want %d", got, want)
	}
	for _, contribution := range payload.Contributions {
		if contribution.Points != 1 {
			t.Fatalf("contribution for %s = %d, want 1", contribution.TileInstanceID, contribution.Points)
		}
	}
	if got, want := len(payload.Awards), 2; got != want {
		t.Fatalf("award count = %d, want %d", got, want)
	}
	if payload.Awards[0].ActorID != "actor-a" || payload.Awards[1].ActorID != "actor-b" {
		t.Fatalf("awards are not deterministic: %#v", payload.Awards)
	}
	for _, award := range payload.Awards {
		if award.Points != payload.TotalPoints {
			t.Fatalf("award for %s = %d, want %d", award.ActorID, award.Points, payload.TotalPoints)
		}
	}
	if got, want := len(payload.ReturnedMeeples), 2; got != want {
		t.Fatalf("returned meeple count = %d, want %d", got, want)
	}
	if payload.ReturnedMeeples[0].ActorID != "actor-b" || payload.ReturnedMeeples[1].ActorID != "actor-a" {
		t.Fatalf("returned meeples = %#v, want original feature order", payload.ReturnedMeeples)
	}
}

func TestCompletedCityAssignsPennantPointsToItsTile(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "city-end-right",
		TileID:     "city_cap",
		X:          1,
		Y:          0,
		Rotation:   270,
		TurnNumber: 9,
	}
	state := carcassonneDTO.GameState{
		TurnNumber: 9,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-end-left", TileID: "city_cap", X: -1, Y: 0, Rotation: 90},
			{InstanceID: "city-shield", TileID: "city_straight_shield", X: 0, Y: 0, Rotation: 0},
			lastPlaced,
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-shield", ZoneID: "city_1", ActorID: "actor-a"},
		},
	}

	result, err := engine.resolveTurn(&state)
	if err != nil {
		t.Fatalf("resolveTurn() error = %v", err)
	}

	if got, want := len(result.Events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	payload := featureScoredPayload(t, result.Events[0])
	if got, want := payload.ScoringPhase, carcassonneDTO.ScoringPhaseTurn; got != want {
		t.Fatalf("scoring phase = %s, want %s", got, want)
	}
	if got, want := payload.TotalPoints, 8; got != want {
		t.Fatalf("total points = %d, want %d", got, want)
	}
	if got, want := scoreContributionTotal(payload.Contributions), payload.TotalPoints; got != want {
		t.Fatalf("contribution total = %d, want %d", got, want)
	}

	pointsByTile := make(map[string]int, len(payload.Contributions))
	for _, contribution := range payload.Contributions {
		pointsByTile[contribution.TileInstanceID] = contribution.Points
	}
	if got, want := pointsByTile["city-end-left"], 2; got != want {
		t.Fatalf("left cap contribution = %d, want %d", got, want)
	}
	if got, want := pointsByTile["city-shield"], 4; got != want {
		t.Fatalf("shield contribution = %d, want %d", got, want)
	}
	if got, want := pointsByTile["city-end-right"], 2; got != want {
		t.Fatalf("right cap contribution = %d, want %d", got, want)
	}
	if got, want := state.Players[0].Score, payload.TotalPoints; got != want {
		t.Fatalf("player score = %d, want %d", got, want)
	}
	if got, want := len(payload.ReturnedMeeples), 1; got != want {
		t.Fatalf("returned meeple count = %d, want %d", got, want)
	}
	if got, want := payload.ReturnedMeeples[0].ActorID, "actor-a"; got != want {
		t.Fatalf("returned meeple actor = %s, want %s", got, want)
	}
}

func TestCompletedMonasteryEmitsOnlyCenterTileContribution(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "surrounding-8",
		TileID:     "monastery",
		X:          1,
		Y:          1,
		TurnNumber: 11,
	}
	state := carcassonneDTO.GameState{
		TurnNumber: 11,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "monastery-center", TileID: "monastery", X: 0, Y: 0},
			{InstanceID: "surrounding-1", TileID: "monastery", X: -1, Y: -1},
			{InstanceID: "surrounding-2", TileID: "monastery", X: 0, Y: -1},
			{InstanceID: "surrounding-3", TileID: "monastery", X: 1, Y: -1},
			{InstanceID: "surrounding-4", TileID: "monastery", X: -1, Y: 0},
			{InstanceID: "surrounding-5", TileID: "monastery", X: 1, Y: 0},
			{InstanceID: "surrounding-6", TileID: "monastery", X: -1, Y: 1},
			{InstanceID: "surrounding-7", TileID: "monastery", X: 0, Y: 1},
			lastPlaced,
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{
				TileInstanceID: "monastery-center",
				ZoneID:         "monastery_1",
				ActorID:        "actor-a",
				FeatureType:    carcassonneDTO.ZoneTypeMonastery,
			},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreCompletedMonasteriesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreCompletedMonasteriesWithEvents() error = %v", err)
	}

	if got, want := len(events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	payload := featureScoredPayload(t, events[0])
	if got, want := payload.ScoringPhase, carcassonneDTO.ScoringPhaseTurn; got != want {
		t.Fatalf("scoring phase = %s, want %s", got, want)
	}
	if got, want := payload.FeatureType, carcassonneDTO.ZoneTypeMonastery; got != want {
		t.Fatalf("feature type = %s, want %s", got, want)
	}
	if got, want := payload.AnchorTileInstanceID, "monastery-center"; got != want {
		t.Fatalf("anchor tile = %s, want %s", got, want)
	}
	if got, want := payload.TotalPoints, 9; got != want {
		t.Fatalf("total points = %d, want %d", got, want)
	}
	if got, want := len(payload.Contributions), 1; got != want {
		t.Fatalf("contribution count = %d, want %d", got, want)
	}
	contribution := payload.Contributions[0]
	if contribution.TileInstanceID != "monastery-center" || contribution.Points != 9 {
		t.Fatalf("contribution = %#v, want +9 on monastery-center", contribution)
	}
	if got, want := len(payload.Awards), 1; got != want {
		t.Fatalf("award count = %d, want %d", got, want)
	}
	if payload.Awards[0].ActorID != "actor-a" || payload.Awards[0].Points != 9 {
		t.Fatalf("award = %#v, want actor-a +9", payload.Awards[0])
	}
	if got, want := len(payload.ReturnedMeeples), 1; got != want {
		t.Fatalf("returned meeple count = %d, want %d", got, want)
	}
	if got, want := state.Players[0].Score, 9; got != want {
		t.Fatalf("player score = %d, want %d", got, want)
	}
	if got := len(state.Meeples); got != 0 {
		t.Fatalf("meeples remaining = %d, want 0", got)
	}
}

func TestResolveTurnAppendsFinalEventsInScoringOrder(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "surrounding-8",
		TileID:     "monastery",
		X:          1,
		Y:          1,
		TurnNumber: 17,
	}
	state := carcassonneDTO.GameState{
		TurnNumber: 17,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 2},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "monastery-complete", TileID: "monastery", X: 0, Y: 0},
			{InstanceID: "surrounding-1", TileID: "monastery", X: -1, Y: -1},
			{InstanceID: "surrounding-2", TileID: "monastery", X: 0, Y: -1},
			{InstanceID: "surrounding-3", TileID: "monastery", X: 1, Y: -1},
			{InstanceID: "surrounding-4", TileID: "monastery", X: -1, Y: 0},
			{InstanceID: "surrounding-5", TileID: "monastery", X: 1, Y: 0},
			{InstanceID: "surrounding-6", TileID: "monastery", X: -1, Y: 1},
			{InstanceID: "surrounding-7", TileID: "monastery", X: 0, Y: 1},
			lastPlaced,
			{InstanceID: "road-final", TileID: "road_straight", X: 10, Y: 0},
			{InstanceID: "city-final", TileID: "city_cap", X: 20, Y: 0},
			{InstanceID: "monastery-final", TileID: "monastery", X: 30, Y: 0},
			{InstanceID: "field-final", TileID: "monastery", X: 40, Y: 0},
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "field-final", ZoneID: "field_1", ActorID: "actor-a"},
			{TileInstanceID: "city-final", ZoneID: "city_1", ActorID: "actor-a"},
			{TileInstanceID: "monastery-final", ZoneID: "monastery_1", ActorID: "actor-a"},
			{TileInstanceID: "road-final", ZoneID: "road_1", ActorID: "actor-a"},
			{TileInstanceID: "monastery-complete", ZoneID: "monastery_1", ActorID: "actor-a"},
		},
	}

	result, err := engine.resolveTurn(&state)
	if err != nil {
		t.Fatalf("resolveTurn() error = %v", err)
	}

	if got, want := len(result.Events), 5; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	wantTypes := []carcassonneDTO.ZoneType{
		carcassonneDTO.ZoneTypeMonastery,
		carcassonneDTO.ZoneTypeRoad,
		carcassonneDTO.ZoneTypeCity,
		carcassonneDTO.ZoneTypeMonastery,
		carcassonneDTO.ZoneTypeField,
	}
	wantPhases := []carcassonneDTO.ScoringPhase{
		carcassonneDTO.ScoringPhaseTurn,
		carcassonneDTO.ScoringPhaseFinal,
		carcassonneDTO.ScoringPhaseFinal,
		carcassonneDTO.ScoringPhaseFinal,
		carcassonneDTO.ScoringPhaseFinal,
	}
	for i, event := range result.Events {
		payload := featureScoredPayload(t, event)
		if payload.FeatureType != wantTypes[i] || payload.ScoringPhase != wantPhases[i] {
			t.Fatalf(
				"event %d = (%s, %s), want (%s, %s)",
				i,
				payload.FeatureType,
				payload.ScoringPhase,
				wantTypes[i],
				wantPhases[i],
			)
		}
	}

	fieldPayload := featureScoredPayload(t, result.Events[4])
	if fieldPayload.TotalPoints != 0 || len(fieldPayload.ScoreMarkers) != 1 {
		t.Fatalf("zero-point field payload = %#v", fieldPayload)
	}
	if fieldPayload.ScoreMarkers[0].Points != 0 {
		t.Fatalf("zero-point field marker = %#v", fieldPayload.ScoreMarkers[0])
	}
	if got, want := state.Players[0].Score, 12; got != want {
		t.Fatalf("final player score = %d, want %d", got, want)
	}
	if got := len(state.Meeples); got != 0 {
		t.Fatalf("meeples remaining = %d, want 0", got)
	}
}

func TestFinalZeroPointExpansionFeaturesStillEmitEvents(t *testing.T) {
	tests := []struct {
		name        string
		tileID      string
		zoneID      string
		featureType carcassonneDTO.ZoneType
	}{
		{
			name:        "incomplete road with inn",
			tileID:      "cottage-replaced-with-tavern-and-lake",
			zoneID:      "road_1",
			featureType: carcassonneDTO.ZoneTypeRoad,
		},
		{
			name:        "incomplete city with cathedral",
			tileID:      "cathedral-with-crypt",
			zoneID:      "city_1",
			featureType: carcassonneDTO.ZoneTypeCity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := testEngine(t)
			state := carcassonneDTO.GameState{
				TurnNumber: 21,
				Players: []carcassonneDTO.PlayerState{
					{ActorID: "actor-a", Score: 4, MeeplesLeft: 6},
				},
				Board: []carcassonneDTO.PlacedTile{
					{InstanceID: "feature", TileID: tt.tileID, X: 0, Y: 0},
				},
				Meeples: []carcassonneDTO.PlacedMeeple{
					{TileInstanceID: "feature", ZoneID: tt.zoneID, ActorID: "actor-a"},
				},
			}

			events := make([]gameService.GameEvent, 0)
			if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
				t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
			}

			if got, want := len(events), 1; got != want {
				t.Fatalf("event count = %d, want %d", got, want)
			}
			payload := featureScoredPayload(t, events[0])
			if payload.FeatureType != tt.featureType || payload.ScoringPhase != carcassonneDTO.ScoringPhaseFinal {
				t.Fatalf("feature metadata = (%s, %s)", payload.FeatureType, payload.ScoringPhase)
			}
			if got := payload.TotalPoints; got != 0 {
				t.Fatalf("total points = %d, want 0", got)
			}
			if got, want := len(payload.Contributions), 1; got != want || payload.Contributions[0].Points != 0 {
				t.Fatalf("zero contributions = %#v, want one tile with 0", payload.Contributions)
			}
			if got, want := len(payload.Awards), 1; got != want || payload.Awards[0].Points != 0 {
				t.Fatalf("zero awards = %#v, want actor award with 0", payload.Awards)
			}
			if got, want := len(payload.ReturnedMeeples), 1; got != want {
				t.Fatalf("returned meeple count = %d, want %d", got, want)
			}
			if got, want := state.Players[0].Score, 4; got != want {
				t.Fatalf("player score = %d, want %d", got, want)
			}
			if got := len(state.Meeples); got != 0 {
				t.Fatalf("meeples remaining = %d, want 0", got)
			}
		})
	}
}

func TestFinalCityAssignsOnePointAndPennantToEachTile(t *testing.T) {
	engine := testEngine(t)
	state := carcassonneDTO.GameState{
		TurnNumber: 23,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-shield", TileID: "city_straight_shield", X: 0, Y: 0},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-shield", ZoneID: "city_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
	}

	if got, want := len(events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	payload := featureScoredPayload(t, events[0])
	if got, want := payload.TotalPoints, 2; got != want {
		t.Fatalf("total points = %d, want %d", got, want)
	}
	if got, want := len(payload.Contributions), 1; got != want {
		t.Fatalf("contribution count = %d, want %d", got, want)
	}
	if got, want := payload.Contributions[0].Points, 2; got != want {
		t.Fatalf("shield tile contribution = %d, want %d", got, want)
	}
}

func TestFinalFieldEventUsesOneMarkerPerWinningActor(t *testing.T) {
	engine := testEngine(t)
	state := carcassonneDTO.GameState{
		TurnNumber: 25,
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 5},
			{ActorID: "actor-b", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-1", TileID: "city_cap", X: 0, Y: 0, Rotation: 0},
			{InstanceID: "city-2", TileID: "city_cap", X: 0, Y: -1, Rotation: 180},
			{InstanceID: "field-1", TileID: "monastery", X: 0, Y: 1},
			{InstanceID: "field-2", TileID: "monastery", X: 1, Y: 1},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "field-2", ZoneID: "field_1", ActorID: "actor-a"},
			{TileInstanceID: "field-1", ZoneID: "field_1", ActorID: "actor-b", MeepleType: carcassonneDTO.MeepleTypeBig},
			{TileInstanceID: "city-1", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
	}

	if got, want := len(events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	payload := featureScoredPayload(t, events[0])
	if payload.FeatureType != carcassonneDTO.ZoneTypeField || payload.ScoringPhase != carcassonneDTO.ScoringPhaseFinal {
		t.Fatalf("field metadata = (%s, %s)", payload.FeatureType, payload.ScoringPhase)
	}
	if got, want := payload.TotalPoints, 3; got != want {
		t.Fatalf("field total points = %d, want %d", got, want)
	}
	if got := len(payload.Contributions); got != 0 {
		t.Fatalf("field contributions = %#v, want empty", payload.Contributions)
	}
	if got, want := len(payload.ContributingCities), 1; got != want {
		t.Fatalf("contributing city count = %d, want %d", got, want)
	}
	city := payload.ContributingCities[0]
	if city.AnchorTileInstanceID != "city-2" || city.AnchorZoneID != "city_1" {
		t.Fatalf("contributing city anchor = %#v, want city-2/city_1", city)
	}
	if got, want := len(city.TileInstanceIDs), 2; got != want {
		t.Fatalf("contributing city tile count = %d, want %d", got, want)
	}
	if city.TileInstanceIDs[0] != "city-2" || city.TileInstanceIDs[1] != "city-1" {
		t.Fatalf("contributing city tiles = %#v, want board order", city.TileInstanceIDs)
	}
	if got, want := len(payload.Awards), 2; got != want {
		t.Fatalf("award count = %d, want %d", got, want)
	}
	if got, want := len(payload.ScoreMarkers), 2; got != want {
		t.Fatalf("score marker count = %d, want %d", got, want)
	}
	if payload.ScoreMarkers[0].ActorID != "actor-a" || payload.ScoreMarkers[0].TileInstanceID != "city-1" {
		t.Fatalf("actor-a marker = %#v, want earliest actor-a farmer", payload.ScoreMarkers[0])
	}
	if payload.ScoreMarkers[1].ActorID != "actor-b" || payload.ScoreMarkers[1].TileInstanceID != "field-1" {
		t.Fatalf("actor-b marker = %#v", payload.ScoreMarkers[1])
	}
	if got, want := payload.AnchorTileInstanceID, "city-1"; got != want {
		t.Fatalf("event anchor = %s, want first marker tile %s", got, want)
	}
	if got, want := len(payload.ReturnedMeeples), 3; got != want {
		t.Fatalf("returned meeple count = %d, want %d", got, want)
	}
	if got, want := state.Players[0].Score, 3; got != want {
		t.Fatalf("actor-a score = %d, want %d", got, want)
	}
	if got, want := state.Players[1].Score, 3; got != want {
		t.Fatalf("actor-b score = %d, want %d", got, want)
	}
}

func TestFinalFieldEventsAreSortedByBoardPosition(t *testing.T) {
	engine := testEngine(t)
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 5},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "field-late", TileID: "monastery", X: 5, Y: 5},
			{InstanceID: "field-first", TileID: "monastery", X: 0, Y: 0},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "field-late", ZoneID: "field_1", ActorID: "actor-a"},
			{TileInstanceID: "field-first", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
	}

	if got, want := len(events), 2; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	first := featureScoredPayload(t, events[0])
	second := featureScoredPayload(t, events[1])
	if first.AnchorTileInstanceID != "field-first" || second.AnchorTileInstanceID != "field-late" {
		t.Fatalf("field event order = [%s, %s], want board order", first.AnchorTileInstanceID, second.AnchorTileInstanceID)
	}
}

func TestFinalRoadsAndCitiesAreSortedByTypeThenBoardPosition(t *testing.T) {
	engine := testEngine(t)
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 3},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "road-late", TileID: "road_straight", X: 10, Y: 10},
			{InstanceID: "city-late", TileID: "city_cap", X: 20, Y: 20},
			{InstanceID: "city-first", TileID: "city_cap", X: 0, Y: 5},
			{InstanceID: "road-first", TileID: "road_straight", X: -10, Y: -10},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-late", ZoneID: "city_1", ActorID: "actor-a"},
			{TileInstanceID: "road-late", ZoneID: "road_1", ActorID: "actor-a"},
			{TileInstanceID: "city-first", ZoneID: "city_1", ActorID: "actor-a"},
			{TileInstanceID: "road-first", ZoneID: "road_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
	}

	wantAnchors := []string{"road-first", "road-late", "city-first", "city-late"}
	if got, want := len(events), len(wantAnchors); got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	for i, event := range events {
		payload := featureScoredPayload(t, event)
		if payload.AnchorTileInstanceID != wantAnchors[i] {
			t.Fatalf("event %d anchor = %s, want %s", i, payload.AnchorTileInstanceID, wantAnchors[i])
		}
	}
}

func TestCompletedCityCanContributeToTwoDifferentFieldEvents(t *testing.T) {
	engine := testEngine(t)
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
			{ActorID: "actor-b", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-top", TileID: "city_cap", X: 0, Y: -1, Rotation: 180},
			{InstanceID: "city-middle", TileID: "city_straight", X: 0, Y: 0},
			{InstanceID: "city-bottom", TileID: "city_cap", X: 0, Y: 1},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-middle", ZoneID: "field_2", ActorID: "actor-b"},
			{TileInstanceID: "city-middle", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	events := make([]gameService.GameEvent, 0)
	if err := engine.scoreFinalFeaturesWithEvents(&state, &events); err != nil {
		t.Fatalf("scoreFinalFeaturesWithEvents() error = %v", err)
	}

	if got, want := len(events), 2; got != want {
		t.Fatalf("field event count = %d, want %d", got, want)
	}
	returned := make(map[string]int)
	for i, event := range events {
		payload := featureScoredPayload(t, event)
		if payload.FeatureType != carcassonneDTO.ZoneTypeField || payload.TotalPoints != 3 {
			t.Fatalf("event %d field score = (%s, %d), want (field, 3)", i, payload.FeatureType, payload.TotalPoints)
		}
		if got, want := len(payload.ContributingCities), 1; got != want {
			t.Fatalf("event %d contributing city count = %d, want %d", i, got, want)
		}
		if got, want := len(payload.ContributingCities[0].TileInstanceIDs), 3; got != want {
			t.Fatalf("event %d contributing city tile count = %d, want %d", i, got, want)
		}
		if got, want := len(payload.ScoreMarkers), 1; got != want {
			t.Fatalf("event %d marker count = %d, want %d", i, got, want)
		}
		if got, want := len(payload.ReturnedMeeples), 1; got != want {
			t.Fatalf("event %d returned meeple count = %d, want %d", i, got, want)
		}
		meeple := payload.ReturnedMeeples[0]
		returned[meeple.ActorID+"/"+meeple.TileInstanceID+"/"+meeple.ZoneID]++
	}

	if got, want := len(returned), 2; got != want {
		t.Fatalf("unique returned meeples = %d, want %d", got, want)
	}
	for key, count := range returned {
		if count != 1 {
			t.Fatalf("returned meeple %s appears %d times, want once", key, count)
		}
	}
	if got, want := state.Players[0].Score, 3; got != want {
		t.Fatalf("actor-a score = %d, want %d", got, want)
	}
	if got, want := state.Players[1].Score, 3; got != want {
		t.Fatalf("actor-b score = %d, want %d", got, want)
	}
}

func featureScoredPayload(t *testing.T, event gameService.GameEvent) carcassonneDTO.FeatureScoredEventPayload {
	t.Helper()

	if event.ID == "" {
		t.Fatal("event ID is empty")
	}
	if event.Type != carcassonneDTO.EventTypeFeatureScored {
		t.Fatalf("event type = %s, want %s", event.Type, carcassonneDTO.EventTypeFeatureScored)
	}

	var payload carcassonneDTO.FeatureScoredEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal feature scored payload: %v", err)
	}
	return payload
}
