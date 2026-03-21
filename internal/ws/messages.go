package ws

import roomDTO "github.com/webmafia/tumladan/internal/room/dto"

type ClientMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type JoinRoomPayload struct {
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
