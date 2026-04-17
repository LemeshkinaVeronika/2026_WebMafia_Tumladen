package model

import "time"

type MatchPlayer struct {
	MatchID        string
	ActorID        string
	DisplayName    string
	Seat           int
	DisconnectedAt *time.Time
}
