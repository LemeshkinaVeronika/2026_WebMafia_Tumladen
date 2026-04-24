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

type PublicGameState struct {
	Version            int            `json:"version"`
	Phase              Phase          `json:"phase"`
	TurnNumber         int            `json:"turnNumber"`
	CurrentPlayerID    string         `json:"currentPlayerId"`
	Players            []PlayerState  `json:"players"`
	Board              []PlacedTile   `json:"board"`
	Meeples            []PlacedMeeple `json:"meeples"`
	CurrentTile        *TileView      `json:"currentTile,omitempty"`
	DeckRemainingCount int            `json:"deckRemaining"`
	Settings           MatchSettings  `json:"settings"`
}

type GameState struct {
	Version         int            `json:"version"`
	Phase           Phase          `json:"phase"`
	TurnNumber      int            `json:"turnNumber"`
	CurrentPlayerID string         `json:"currentPlayerId"`
	Players         []PlayerState  `json:"players"`
	Board           []PlacedTile   `json:"board"`
	Settings        MatchSettings  `json:"settings"`
	DeckRemaining   []TileInstance `json:"deckRemaining"`
	CurrentTile     *TileInstance  `json:"currentTile,omitempty"`
	LastPlacedTile  *PlacedTile    `json:"lastPlacedTile,omitempty"`
	Meeples         []PlacedMeeple `json:"meeples"`
}

type TileInstance struct {
	InstanceID string `json:"instanceId"`
	TileID     string `json:"tileId"`
}

type PlacedTile struct {
	InstanceID string `json:"instanceId"`
	TileID     string `json:"tileId"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Rotation   int    `json:"rotation"`
	PlacedBy   string `json:"placedBy"`
	TurnNumber int    `json:"turnNumber"`
}

type PlacedMeeple struct {
	TileInstanceID string `json:"tileInstanceId"`
	ZoneID         string `json:"zoneId"`
	ActorID        string `json:"actorId"`
}

type TileView struct {
	TileID   string `json:"tileId"`
	ImageKey string `json:"imageKey"`
}

type PrivateGameState struct {
	IsYourTurn            bool                   `json:"isYourTurn"`
	Phase                 string                 `json:"phase"`
	CurrentPlayerID       string                 `json:"currentPlayerId"`
	AllowedTilePlacements []AllowedTilePlacement `json:"allowedTilePlacements,omitempty"`
	AllowedMeepleZones    []string               `json:"allowedMeepleZones,omitempty"`
	CanSkipMeeple         bool                   `json:"canSkipMeeple"`
}

type AllowedTilePlacement struct {
	X        int `json:"x"`
	Y        int `json:"y"`
	Rotation int `json:"rotation"`
}
