package model

import "time"

type MatchStatus string
type MatchTerminationReason string
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
