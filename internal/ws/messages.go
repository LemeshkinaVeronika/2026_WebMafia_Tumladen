package ws

import (
	"encoding/json"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

type ClientMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type JoinRoomPayload struct {
	RoomID string `json:"roomId"`
}

type UpdateRoomSettingsPayload struct {
	RoomID     string          `json:"roomId"`
	GameType   string          `json:"gameType"`
	MaxPlayers int             `json:"maxPlayers"`
	Settings   json.RawMessage `json:"settings"`
}

type StartRoomPayload struct {
	RoomID string `json:"roomId"`
}

type ServerMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

type RoomStatePayload = roomDTO.RoomResponse
