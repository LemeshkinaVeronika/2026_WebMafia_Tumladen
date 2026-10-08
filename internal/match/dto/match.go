package dto

import "encoding/json"

import gameService "github.com/webmafia/tumladan/internal/game/service"

type MatchPlayerResponse struct {
	ActorID        string `json:"actorId"`
	ActorType      string `json:"actorType"`
	DisplayName    string `json:"displayName"`
	BotDifficulty  string `json:"botDifficulty,omitempty"`
	AvatarURL      string `json:"avatarUrl,omitempty"`
	Seat           int    `json:"seat"`
	IsDisconnected bool   `json:"isDisconnected"`
}

type MatchActivityResponse struct {
	ID           string          `json:"id"`
	StateVersion int             `json:"stateVersion"`
	TurnNumber   int             `json:"turnNumber"`
	ActorID      string          `json:"actorId"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    string          `json:"createdAt"`
}

type MatchResponse struct {
	ID                  string                  `json:"id"`
	RoomID              string                  `json:"roomId"`
	GameType            string                  `json:"gameType"`
	Status              string                  `json:"status"`
	GameState           json.RawMessage         `json:"gameState"`
	IsYourTurn          *bool                   `json:"isYourTurn,omitempty"`
	Result              json.RawMessage         `json:"result,omitempty"`
	TerminationReason   *string                 `json:"terminationReason,omitempty"`
	TerminatedByActorID *string                 `json:"terminatedByActorId,omitempty"`
	TerminatedAt        *string                 `json:"terminatedAt,omitempty"`
	Players             []MatchPlayerResponse   `json:"players"`
	CreatedAt           string                  `json:"createdAt"`
	UpdatedAt           string                  `json:"updatedAt"`
	Events              []gameService.GameEvent `json:"events,omitempty"`
	RecentActions       []MatchActivityResponse `json:"recentActions"`
}

type ApplyMatchActionRequest struct {
	ActionID             string          `json:"actionId"`
	ActorID              string          `json:"actorId"`
	RoomID               string          `json:"roomId"`
	ExpectedMatchID      string          `json:"expectedMatchId"`
	Action               string          `json:"action"`
	ExpectedTurnNumber   int             `json:"expectedTurnNumber"`
	ExpectedPhase        string          `json:"expectedPhase"`
	ExpectedStateVersion int             `json:"expectedStateVersion"`
	Payload              json.RawMessage `json:"payload"`
}

type ApplyMatchActionResult struct {
	MatchState   *MatchResponse
	Status       string
	StateVersion int
	ErrorCode    string
	ErrorMessage string
	Replayed     bool
}

type ApplyTurnTimeoutRequest struct {
	RoomID               string `json:"roomId"`
	ExpectedMatchID      string `json:"expectedMatchId"`
	ExpectedActorID      string `json:"expectedActorId"`
	ExpectedPhase        string `json:"expectedPhase"`
	ExpectedTurnNumber   int    `json:"expectedTurnNumber"`
	ExpectedStateVersion int    `json:"expectedStateVersion"`
}
