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
	IsPrivate  *bool           `json:"isPrivate,omitempty"`
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
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	ErrorInvalidPayload     = "INVALID_PAYLOAD"
	ErrorRoomNotFound       = "ROOM_NOT_FOUND"
	ErrorRoomFull           = "ROOM_FULL"
	ErrorRoomNotJoinable    = "ROOM_NOT_JOINABLE"
	ErrorForbidden          = "FORBIDDEN"
	ErrorNotYourTurn        = "NOT_YOUR_TURN"
	ErrorInvalidMatchAction = "INVALID_MATCH_ACTION"
	ErrorMatchNotFound      = "MATCH_NOT_FOUND"
	ErrorMatchNotActive     = "MATCH_NOT_ACTIVE"
	ErrorMatchStateConflict = "MATCH_STATE_CONFLICT"
	ErrorActionIDConflict   = "ACTION_ID_CONFLICT"
	ErrorInternal           = "INTERNAL_ERROR"
)

type ParticipantKickedPayload struct {
	Reason string `json:"reason"`
}

type RoomStatePayload = roomDTO.RoomResponse

type MatchStatePayload = matchDTO.MatchResponse

type MatchActionPayload struct {
	ActionID             string `json:"actionId"`
	RoomID               string `json:"roomId"`
	ExpectedMatchID      string `json:"expectedMatchId"`
	Action               string `json:"action"`
	ExpectedTurnNumber   int    `json:"expectedTurnNumber"`
	ExpectedPhase        string `json:"expectedPhase"`
	ExpectedStateVersion int    `json:"expectedStateVersion"`
	Payload              any    `json:"payload"`
}

type MatchActionResultPayload struct {
	ActionID     string        `json:"actionId"`
	Status       string        `json:"status"`
	StateVersion *int          `json:"stateVersion,omitempty"`
	Error        *ErrorPayload `json:"error,omitempty"`
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
