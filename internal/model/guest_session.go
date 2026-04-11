package model

import "time"

type GuestSession struct {
	SessionID   string
	ActorID     string
	DisplayName string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}
