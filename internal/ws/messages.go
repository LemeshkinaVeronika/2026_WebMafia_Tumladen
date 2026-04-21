package ws

import (
	"encoding/json"

	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

type ClientMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type JoinRoomPayload struct {
	RoomID string `json:"roomId"`
}

type LeaveRoomPayload struct {
	RoomID string `json:"roomId"`
}

type UpdateRoomSettingsPayload struct {
	Name       string          `json:"name"`
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

type ParticipantKickedPayload struct {
	Reason string `json:"reason"`
}

type RoomStatePayload = roomDTO.RoomResponse

type MatchStatePayload = matchDTO.MatchResponse

type MatchActionPayload struct {
	RoomID  string `json:"roomId"`
	Action  string `json:"action"`
	Payload any    `json:"payload"`
}

type DeleteRoomPayload struct {
	RoomID string `json:"roomId"`
}

type LeaveMatchPayload struct {
	RoomID string `json:"roomId"`
}

type KickParticipantPayload struct {
	RoomID        string `json:"roomId"`
	TargetActorID string `json:"targetActorId"`
}
