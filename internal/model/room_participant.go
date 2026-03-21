package model

import "time"

type RoomParticipantView struct {
	RoomID      string
	ActorID     string
	DisplayName string
	JoinedAt    time.Time
}
