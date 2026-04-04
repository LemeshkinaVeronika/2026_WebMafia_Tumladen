package model

import "time"

type RoomParticipant struct {
	RoomID      string
	ActorID     string
	DisplayName string
	JoinedAt    time.Time
}
