package model

import "time"

type MatchStatus string
type MatchTerminationReason string
type MatchActionStatus string
type JSONB []byte

const (
	MatchStatusPending   MatchStatus = "pending"
	MatchStatusActive    MatchStatus = "active"
	MatchStatusFinished  MatchStatus = "finished"
	MatchStatusAbandoned MatchStatus = "abandoned"
)

const (
	MatchTerminationReasonNormalCompletion MatchTerminationReason = "normal_completion"
	MatchTerminationReasonPlayerLeft       MatchTerminationReason = "player_left"
	MatchTerminationReasonReconnectTimeout MatchTerminationReason = "reconnect_timeout"
	MatchTerminationReasonRoomDeleted      MatchTerminationReason = "room_deleted"
)

const (
	MatchActionStatusAccepted MatchActionStatus = "accepted"
	MatchActionStatusRejected MatchActionStatus = "rejected"
)

type Match struct {
	ID                  string
	RoomID              string
	GameType            string
	Status              MatchStatus
	GameState           JSONB
	Result              *JSONB
	TerminationReason   *MatchTerminationReason
	TerminatedByActorID *string
	TerminatedAt        *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MatchWithPlayers struct {
	Match   Match
	Players []MatchPlayer
}

// MatchActionReceipt is the durable result of a client-issued match command.
// The request hash prevents an action ID from being reused for different input.
type MatchActionReceipt struct {
	RoomID       string
	MatchID      string
	ActorID      string
	ActionID     string
	RequestHash  string
	Status       MatchActionStatus
	StateVersion int
	ErrorCode    string
	ErrorMessage string
	CreatedAt    time.Time
}
