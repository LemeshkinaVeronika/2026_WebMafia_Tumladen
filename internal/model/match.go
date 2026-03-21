package model

import "time"

type MatchStatus string
type JSONB []byte

const (
	MatchStatusPending   MatchStatus = "pending"
	MatchStatusActive    MatchStatus = "active"
	MatchStatusFinished  MatchStatus = "finished"
	MatchStatusAbandoned MatchStatus = "abandoned"
)

type Match struct {
	ID        string
	RoomID    string
	GameType  string
	Status    MatchStatus
	GameState JSONB
	Result    JSONB
	CreatedAt time.Time
	UpdatedAt time.Time
}
