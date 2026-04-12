package dto

import "encoding/json"

type MatchPlayerResponse struct {
	ActorID     string `json:"actorId"`
	DisplayName string `json:"displayName"`
	Seat        int    `json:"seat"`
}

type MatchResponse struct {
	ID         string                `json:"id"`
	RoomID     string                `json:"roomId"`
	GameType   string                `json:"gameType"`
	Status     string                `json:"status"`
	GameState  json.RawMessage       `json:"gameState"`
	IsYourTurn *bool                 `json:"isYourTurn,omitempty"`
	Result     json.RawMessage       `json:"result,omitempty"`
	Players    []MatchPlayerResponse `json:"players"`
	CreatedAt  string                `json:"createdAt"`
	UpdatedAt  string                `json:"updatedAt"`
}

type ApplyMatchActionRequest struct {
	ActorID string          `json:"actorId"`
	RoomID  string          `json:"roomId"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}
