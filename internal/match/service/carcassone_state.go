package service

type CarcassonneMatchSettings struct {
	TurnTimeSeconds int `json:"turnTimeSeconds"`
}

type CarcassonnePlayerState struct {
	ActorID     string `json:"actorId"`
	DisplayName string `json:"displayName"`
	Seat        int    `json:"seat"`
	Score       int    `json:"score"`
	MeeplesLeft int    `json:"meeplesLeft"`
}

type CarcassonneGameState struct {
	Version         int                      `json:"version"`
	Phase           string                   `json:"phase"`
	TurnNumber      int                      `json:"turnNumber"`
	CurrentPlayerID string                   `json:"currentPlayerId"`
	Players         []CarcassonnePlayerState `json:"players"`
	Board           []any                    `json:"board"`
	CurrentTile     any                      `json:"currentTile,omitempty"`
	DeckRemaining   int                      `json:"deckRemaining"`
	Settings        CarcassonneMatchSettings `json:"settings"`
}
