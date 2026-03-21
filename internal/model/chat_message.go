package model

import "time"

type ChatMessage struct {
	ID          string
	RoomID      string
	ActorID     string
	DisplayName string
	Body        string
	CreatedAt   time.Time
}
