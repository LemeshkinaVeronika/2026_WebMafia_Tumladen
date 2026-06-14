package model

import "time"

type RoomParticipant struct {
	RoomID        string
	ActorID       string
	ActorType     ActorType
	DisplayName   string
	BotDifficulty string
	JoinedAt      time.Time
}
