package model

import "time"

// MatchActivity is a durable, structured fact produced by a successful game
// action. Unlike transient game events, activities are persisted and may be
// replayed to clients after reconnecting.
type MatchActivity struct {
	ID           string
	MatchID      string
	StateVersion int
	Ordinal      int
	TurnNumber   int
	ActorID      string
	Type         string
	Payload      JSONB
	CreatedAt    time.Time
}
