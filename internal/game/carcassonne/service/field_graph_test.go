package service

import (
	"testing"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()

	catalog, err := NewAssetTileCatalog()
	if err != nil {
		t.Fatalf("NewAssetTileCatalog() error = %v", err)
	}

	return &Engine{catalog: catalog}
}

func TestCanPlaceMeepleRejectsOccupiedFieldThroughDSU(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "tile-2",
		TileID:     "monastery",
		X:          1,
		Y:          0,
		Rotation:   0,
	}
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "tile-1", TileID: "monastery", X: 0, Y: 0, Rotation: 0},
			lastPlaced,
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "tile-1", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	if engine.canPlaceMeeple(state, "actor-b", "field_1") {
		t.Fatal("canPlaceMeeple() allowed a farmer on an already occupied DSU field")
	}
}

func TestCanPlaceMeepleRejectsOccupiedFieldThroughCurveCityRoadField(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "candidate",
		TileID:     "road_curve",
		X:          1,
		Y:          0,
		Rotation:   0,
	}
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "bridge", TileID: "city_curve_with_road_curve", X: 0, Y: 0, Rotation: 0},
			{InstanceID: "occupied", TileID: "road_straight", X: 0, Y: 1, Rotation: 0},
			lastPlaced,
		},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "occupied", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	if engine.canPlaceMeeple(state, "actor-b", "field_1") {
		t.Fatal("canPlaceMeeple() allowed a farmer through a split field on city_curve_with_road_curve")
	}
}

func TestFinalFieldScoringCountsCompletedCityOnce(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-1", TileID: "city_cap", X: 0, Y: 0, Rotation: 0},
			{InstanceID: "city-2", TileID: "city_cap", X: 0, Y: -1, Rotation: 180},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-1", ZoneID: "field_1", ActorID: "actor-a"},
		},
	}

	if err := engine.scoreFinalFeatures(&state); err != nil {
		t.Fatalf("scoreFinalFeatures() error = %v", err)
	}

	if got, want := state.Players[0].Score, 3; got != want {
		t.Fatalf("final farmer score = %d, want %d", got, want)
	}
}

func TestFinalFieldScoringTiesPlayersOnSameField(t *testing.T) {
	engine := testEngine(t)

	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
			{ActorID: "actor-b", MeeplesLeft: 6},
		},
		Board: []carcassonneDTO.PlacedTile{
			{InstanceID: "city-1", TileID: "city_cap", X: 0, Y: 0, Rotation: 0},
			{InstanceID: "city-2", TileID: "city_cap", X: 0, Y: -1, Rotation: 180},
			{InstanceID: "field-1", TileID: "monastery", X: 0, Y: 1, Rotation: 0},
		},
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "city-1", ZoneID: "field_1", ActorID: "actor-a"},
			{TileInstanceID: "field-1", ZoneID: "field_1", ActorID: "actor-b"},
		},
	}

	if err := engine.scoreFinalFeatures(&state); err != nil {
		t.Fatalf("scoreFinalFeatures() error = %v", err)
	}

	if got, want := state.Players[0].Score, 3; got != want {
		t.Fatalf("actor-a final farmer score = %d, want %d", got, want)
	}
	if got, want := state.Players[1].Score, 3; got != want {
		t.Fatalf("actor-b final farmer score = %d, want %d", got, want)
	}
}

func TestRoadTJunctionBranchesAreSeparateFeatures(t *testing.T) {
	engine := testEngine(t)

	lastPlaced := carcassonneDTO.PlacedTile{
		InstanceID: "junction",
		TileID:     "road_t",
		X:          0,
		Y:          0,
		Rotation:   0,
	}
	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", MeeplesLeft: 6},
			{ActorID: "actor-b", MeeplesLeft: 7},
		},
		Board:          []carcassonneDTO.PlacedTile{lastPlaced},
		LastPlacedTile: &lastPlaced,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "junction", ZoneID: "road_left", ActorID: "actor-a"},
		},
	}

	if !engine.canPlaceMeeple(state, "actor-b", "road_right") {
		t.Fatal("canPlaceMeeple() rejected a meeple on a different road branch")
	}
}

func TestRoadTileFieldZonesAreSingleConnectedFields(t *testing.T) {
	engine := testEngine(t)

	for _, def := range engine.catalog.Definitions() {
		if !tileHasRoad(def) {
			continue
		}

		t.Run(def.TileID, func(t *testing.T) {
			tile := carcassonneDTO.PlacedTile{
				InstanceID: "tile",
				TileID:     def.TileID,
				X:          0,
				Y:          0,
				Rotation:   0,
			}
			graph, err := engine.buildFieldGraph(carcassonneDTO.GameState{
				Board: []carcassonneDTO.PlacedTile{tile},
			})
			if err != nil {
				t.Fatalf("buildFieldGraph() error = %v", err)
			}

			for _, zone := range def.Zones {
				if zone.Type != carcassonneDTO.ZoneTypeField {
					continue
				}

				roots := graph.rootsForZone(placedZoneRef{TileInstanceID: tile.InstanceID, ZoneID: zone.ZoneID})
				if len(roots) != 1 {
					t.Fatalf("field zone %s has %d connected field roots, want 1", zone.ZoneID, len(roots))
				}
			}
		})
	}
}

func TestRoadTileRoadEdgesMatchRoadZoneSegments(t *testing.T) {
	engine := testEngine(t)

	for _, def := range engine.catalog.Definitions() {
		if !tileHasRoad(def) {
			continue
		}

		t.Run(def.TileID, func(t *testing.T) {
			roadSegmentCounts := make(map[carcassonneDTO.ZoneSegment]int)
			for _, zone := range def.Zones {
				if zone.Type != carcassonneDTO.ZoneTypeRoad {
					continue
				}
				for _, segment := range zone.Segments {
					roadSegmentCounts[segment]++
				}
			}

			expectedRoadEdges := map[carcassonneDTO.ZoneSegment]bool{
				carcassonneDTO.SegmentTopCenter:    def.Edges.Top == carcassonneDTO.EdgeTypeRoad,
				carcassonneDTO.SegmentRightCenter:  def.Edges.Right == carcassonneDTO.EdgeTypeRoad,
				carcassonneDTO.SegmentBottomCenter: def.Edges.Bottom == carcassonneDTO.EdgeTypeRoad,
				carcassonneDTO.SegmentLeftCenter:   def.Edges.Left == carcassonneDTO.EdgeTypeRoad,
			}

			for segment, expected := range expectedRoadEdges {
				got := roadSegmentCounts[segment]
				if expected && got != 1 {
					t.Fatalf("road edge segment %s count = %d, want 1", segment, got)
				}
				if !expected && got != 0 {
					t.Fatalf("non-road edge segment %s count = %d, want 0", segment, got)
				}
			}
		})
	}
}

func TestRoadJunctionBranchesAreSeparateFeaturesForAllJunctionTiles(t *testing.T) {
	engine := testEngine(t)

	for _, def := range engine.catalog.Definitions() {
		roadZones := roadZones(def)
		if len(roadZones) < 2 {
			continue
		}

		t.Run(def.TileID, func(t *testing.T) {
			lastPlaced := carcassonneDTO.PlacedTile{
				InstanceID: "tile",
				TileID:     def.TileID,
				X:          0,
				Y:          0,
				Rotation:   0,
			}
			state := carcassonneDTO.GameState{
				Players: []carcassonneDTO.PlayerState{
					{ActorID: "actor-a", MeeplesLeft: 6},
					{ActorID: "actor-b", MeeplesLeft: 7},
				},
				Board:          []carcassonneDTO.PlacedTile{lastPlaced},
				LastPlacedTile: &lastPlaced,
				Meeples: []carcassonneDTO.PlacedMeeple{
					{TileInstanceID: lastPlaced.InstanceID, ZoneID: roadZones[0].ZoneID, ActorID: "actor-a"},
				},
			}

			for _, zone := range roadZones[1:] {
				if !engine.canPlaceMeeple(state, "actor-b", zone.ZoneID) {
					t.Fatalf("canPlaceMeeple() rejected road branch %s with occupied branch %s", zone.ZoneID, roadZones[0].ZoneID)
				}
			}
		})
	}
}

func TestScoreCompletedMonasteriesDoesNotSkipAfterRemovingMeeple(t *testing.T) {
	engine := testEngine(t)

	board := []carcassonneDTO.PlacedTile{
		{InstanceID: "monastery-1", TileID: "monastery", X: 0, Y: 0, Rotation: 0},
		{InstanceID: "monastery-2", TileID: "monastery", X: 10, Y: 0, Rotation: 0},
	}
	board = append(board, surroundingTiles("m1", 0, 0)...)
	board = append(board, surroundingTiles("m2", 10, 0)...)

	state := carcassonneDTO.GameState{
		Players: []carcassonneDTO.PlayerState{
			{ActorID: "actor-a", Score: 0, MeeplesLeft: 5},
		},
		Board: board,
		Meeples: []carcassonneDTO.PlacedMeeple{
			{TileInstanceID: "monastery-1", ZoneID: "monastery_1", ActorID: "actor-a"},
			{TileInstanceID: "monastery-2", ZoneID: "monastery_1", ActorID: "actor-a"},
		},
	}

	if err := engine.scoreCompletedMonasteries(&state); err != nil {
		t.Fatalf("scoreCompletedMonasteries() error = %v", err)
	}

	if got, want := state.Players[0].Score, 18; got != want {
		t.Fatalf("score after two monasteries = %d, want %d", got, want)
	}
	if got, want := state.Players[0].MeeplesLeft, 7; got != want {
		t.Fatalf("meeples after two monasteries = %d, want %d", got, want)
	}
	if len(state.Meeples) != 0 {
		t.Fatalf("meeples left on board = %d, want 0", len(state.Meeples))
	}
}

func TestDrawNextPlaceableTileReturnsSkippedTilesToDeck(t *testing.T) {
	engine := testEngine(t)

	deck := []carcassonneDTO.TileInstance{
		{InstanceID: "blocked", TileID: "road_cross"},
		{InstanceID: "placeable", TileID: "city_cap"},
		{InstanceID: "tail", TileID: "monastery"},
	}
	board := []carcassonneDTO.PlacedTile{
		{InstanceID: "city", TileID: "city_full_shield", X: 0, Y: 0, Rotation: 0},
	}

	tile, remaining := engine.drawNextPlaceableTile(deck, board)

	if tile == nil || tile.InstanceID != "placeable" {
		t.Fatalf("drawNextPlaceableTile() tile = %#v, want placeable", tile)
	}
	if got, want := len(remaining), 2; got != want {
		t.Fatalf("remaining deck length = %d, want %d", got, want)
	}
	if remaining[0].InstanceID != "tail" || remaining[1].InstanceID != "blocked" {
		t.Fatalf("remaining deck order = %#v, want tail then skipped blocked", remaining)
	}
}

func TestDrawNextPlaceableTileKeepsDeckWhenNoTileIsPlaceable(t *testing.T) {
	engine := testEngine(t)

	deck := []carcassonneDTO.TileInstance{
		{InstanceID: "blocked-1", TileID: "road_cross"},
		{InstanceID: "blocked-2", TileID: "road_t"},
	}
	board := []carcassonneDTO.PlacedTile{
		{InstanceID: "city", TileID: "city_full_shield", X: 0, Y: 0, Rotation: 0},
	}

	tile, remaining := engine.drawNextPlaceableTile(deck, board)

	if tile != nil {
		t.Fatalf("drawNextPlaceableTile() tile = %#v, want nil", tile)
	}
	if got, want := len(remaining), len(deck); got != want {
		t.Fatalf("remaining deck length = %d, want %d", got, want)
	}
	for i := range deck {
		if remaining[i].InstanceID != deck[i].InstanceID {
			t.Fatalf("remaining[%d] = %s, want %s", i, remaining[i].InstanceID, deck[i].InstanceID)
		}
	}
}

func tileHasRoad(def carcassonneDTO.TileDefinition) bool {
	if def.Edges.Top == carcassonneDTO.EdgeTypeRoad ||
		def.Edges.Right == carcassonneDTO.EdgeTypeRoad ||
		def.Edges.Bottom == carcassonneDTO.EdgeTypeRoad ||
		def.Edges.Left == carcassonneDTO.EdgeTypeRoad {
		return true
	}

	return len(roadZones(def)) > 0
}

func roadZones(def carcassonneDTO.TileDefinition) []carcassonneDTO.ZoneDefinition {
	result := make([]carcassonneDTO.ZoneDefinition, 0)
	for _, zone := range def.Zones {
		if zone.Type == carcassonneDTO.ZoneTypeRoad {
			result = append(result, zone)
		}
	}
	return result
}

func surroundingTiles(prefix string, x, y int) []carcassonneDTO.PlacedTile {
	coordinates := [][2]int{
		{x - 1, y - 1},
		{x, y - 1},
		{x + 1, y - 1},
		{x - 1, y},
		{x + 1, y},
		{x - 1, y + 1},
		{x, y + 1},
		{x + 1, y + 1},
	}

	tiles := make([]carcassonneDTO.PlacedTile, 0, len(coordinates))
	for i, coordinate := range coordinates {
		tiles = append(tiles, carcassonneDTO.PlacedTile{
			InstanceID: prefix + "-surrounding-" + string(rune('a'+i)),
			TileID:     "monastery",
			X:          coordinate[0],
			Y:          coordinate[1],
			Rotation:   0,
		})
	}

	return tiles
}
