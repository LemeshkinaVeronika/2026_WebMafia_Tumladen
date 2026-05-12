package dto

import "encoding/json"

type MatchPlayerResponse struct {
	ActorID        string `json:"actorId"`
	ActorType      string `json:"actorType"`
	DisplayName    string `json:"displayName"`
	Seat           int    `json:"seat"`
	IsDisconnected bool   `json:"isDisconnected"`
}

type MatchResponse struct {
	ID                  string                `json:"id"`
	RoomID              string                `json:"roomId"`
	GameType            string                `json:"gameType"`
	Status              string                `json:"status"`
	GameState           json.RawMessage       `json:"gameState"`
	IsYourTurn          *bool                 `json:"isYourTurn,omitempty"`
	Result              json.RawMessage       `json:"result,omitempty"`
	TerminationReason   *string               `json:"terminationReason,omitempty"`
	TerminatedByActorID *string               `json:"terminatedByActorId,omitempty"`
	TerminatedAt        *string               `json:"terminatedAt,omitempty"`
	Players             []MatchPlayerResponse `json:"players"`
	CreatedAt           string                `json:"createdAt"`
	UpdatedAt           string                `json:"updatedAt"`
}

type ApplyMatchActionRequest struct {
	ActorID string          `json:"actorId"`
	RoomID  string          `json:"roomId"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type ApplyTurnTimeoutRequest struct {
	RoomID               string `json:"roomId"`
	ExpectedMatchID      string `json:"expectedMatchId"`
	ExpectedActorID      string `json:"expectedActorId"`
	ExpectedPhase        string `json:"expectedPhase"`
	ExpectedTurnNumber   int    `json:"expectedTurnNumber"`
	ExpectedStateVersion int    `json:"expectedStateVersion"`
}
