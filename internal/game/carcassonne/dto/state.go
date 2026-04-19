package dto

type MatchSettings struct {
	TurnTimeSeconds int `json:"turnTimeSeconds"`
}

type PlayerState struct {
	ActorID     string `json:"actorId"`
	DisplayName string `json:"displayName"`
	Seat        int    `json:"seat"`
	Score       int    `json:"score"`
	MeeplesLeft int    `json:"meeplesLeft"`
}

type Tile struct {
	ID string `json:"id"`
}

type PlacedTile struct {
	TileID     string `json:"tileId"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Rotation   int    `json:"rotation"`
	PlacedBy   string `json:"placedBy"`
	TurnNumber int    `json:"turnNumber"`
}

type GameState struct {
	Version         int           `json:"version"`
	Phase           string        `json:"phase"`
	TurnNumber      int           `json:"turnNumber"`
	CurrentPlayerID string        `json:"currentPlayerId"`
	Players         []PlayerState `json:"players"`
	Board           []PlacedTile  `json:"board"`
	CurrentTile     *Tile         `json:"currentTile,omitempty"`
	DeckRemaining   int           `json:"deckRemaining"`
	Settings        MatchSettings `json:"settings"`
}

type PublicGameState struct {
	Version         int           `json:"version"`
	Phase           string        `json:"phase"`
	TurnNumber      int           `json:"turnNumber"`
	CurrentPlayerID string        `json:"currentPlayerId"`
	Players         []PlayerState `json:"players"`
	Board           []PlacedTile  `json:"board"`
	CurrentTile     *Tile         `json:"currentTile,omitempty"`
	DeckRemaining   int           `json:"deckRemaining"`
	Settings        MatchSettings `json:"settings"`
}
