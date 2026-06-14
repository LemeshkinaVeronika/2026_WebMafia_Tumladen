package model

import "time"

type MatchPlayer struct {
	MatchID        string
	ActorID        string
	ActorType      ActorType
	DisplayName    string
	BotDifficulty  string
	AvatarURL      string
	Seat           int
	DisconnectedAt *time.Time
}
