package dto

type MatchSettings struct {
	TurnTimeSeconds int      `json:"turnTimeSeconds"`
	Expansions      []string `json:"expansions,omitempty"`
}

type PlayerState struct {
	ActorID        string `json:"actorId"`
	DisplayName    string `json:"displayName"`
	Seat           int    `json:"seat"`
	Score          int    `json:"score"`
	MeeplesLeft    int    `json:"meeplesLeft"`
	BigMeeplesLeft int    `json:"bigMeeplesLeft,omitempty"`
}

type PublicGameState struct {
	Version         int              `json:"version"`
	Status          string           `json:"status"`
	Phase           Phase            `json:"phase"`
	TurnNumber      int              `json:"turnNumber"`
	CurrentPlayerID *string          `json:"currentPlayerId"`
	Players         []PlayerState    `json:"players"`
	CurrentTurn     CurrentTurnState `json:"currentTurn"`
	Deck            DeckState        `json:"deck"`
	Board           BoardState       `json:"board"`
	Meeples         []PlacedMeeple   `json:"meeples"`
	Settings        MatchSettings    `json:"settings"`
	Result          *MatchResult     `json:"result,omitempty"`
}

type GameState struct {
	Version         int            `json:"version"`
	Phase           Phase          `json:"phase"`
	TurnNumber      int            `json:"turnNumber"`
	TurnStartedAt   string         `json:"turnStartedAt,omitempty"`
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
	TileInstanceID string      `json:"tileInstanceId"`
	ZoneID         string      `json:"zoneId"`
	ActorID        string      `json:"actorId"`
	MeepleType     MeepleType  `json:"meepleType,omitempty"`
	FeatureType    ZoneType    `json:"featureType,omitempty"`
	Segment        ZoneSegment `json:"segment,omitempty"`
}

type MeepleType string

const (
	MeepleTypeRegular MeepleType = "regular"
	MeepleTypeBig     MeepleType = "big"
)

type TileView struct {
	TileID   string `json:"tileId"`
	ImageURL string `json:"imageUrl"`
}

type CurrentTurnState struct {
	DrawnTile    *TileView   `json:"drawnTile"`
	PlacedTile   *PlacedTile `json:"placedTile"`
	MeeplePlaced bool        `json:"meeplePlaced"`
	TurnEndsAt   *string     `json:"turnEndsAt,omitempty"`
}

type DeckState struct {
	RemainingCount int `json:"remainingCount"`
}

type BoardState struct {
	Tiles []PlacedTile `json:"tiles"`
}

type PrivateGameState struct {
	IsYourTurn            bool                   `json:"isYourTurn"`
	Phase                 Phase                  `json:"phase"`
	CurrentPlayerID       *string                `json:"currentPlayerId"`
	AllowedActions        []Action               `json:"allowedActions"`
	ValidPlacements       []ValidTilePlacement   `json:"validPlacements"`
	ValidMeeplePlacements []ValidMeeplePlacement `json:"validMeeplePlacements"`
}

type ValidTilePlacement struct {
	X         int   `json:"x"`
	Y         int   `json:"y"`
	Rotations []int `json:"rotations"`
}

type ValidMeeplePlacement struct {
	TileInstanceID       string       `json:"tileInstanceId"`
	X                    int          `json:"x"`
	Y                    int          `json:"y"`
	ZoneID               string       `json:"zoneId"`
	FeatureType          ZoneType     `json:"featureType"`
	Segment              ZoneSegment  `json:"segment"`
	AvailableMeepleTypes []MeepleType `json:"availableMeepleTypes,omitempty"`
}

type MatchResult struct {
	Winners     []string     `json:"winners"`
	FinalScores []FinalScore `json:"finalScores"`
}

type FinalScore struct {
	ActorID string `json:"actorId"`
	Score   int    `json:"score"`
}
