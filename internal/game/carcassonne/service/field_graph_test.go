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
		{InstanceID: "start", TileID: "start_tile", X: 0, Y: 0, Rotation: 0},
	}

	tile, remaining := engine.drawNextPlaceableTile(deck, board)

	if tile == nil || tile.InstanceID != "placeable" {
		t.Fatalf("drawNextPlaceableTile() tile = %#v, want placeable", tile)
	}
	if got, want := len(remaining), 2; got != want {
		t.Fatalf("remaining deck length = %d, want %d", got, want)
	}
	if remaining[0].InstanceID != "blocked" || remaining[1].InstanceID != "tail" {
		t.Fatalf("remaining deck order = %#v, want skipped blocked then tail", remaining)
	}
}

func TestDrawNextPlaceableTileKeepsDeckWhenNoTileIsPlaceable(t *testing.T) {
	engine := testEngine(t)

	deck := []carcassonneDTO.TileInstance{
		{InstanceID: "blocked-1", TileID: "road_cross"},
		{InstanceID: "blocked-2", TileID: "road_t"},
	}
	board := []carcassonneDTO.PlacedTile{
		{InstanceID: "start", TileID: "monastery", X: 0, Y: 0, Rotation: 0},
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
