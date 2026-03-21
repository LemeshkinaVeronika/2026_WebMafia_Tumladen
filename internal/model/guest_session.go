package model

import "time"

type GuestSession struct {
	ActorID     string
	DisplayName string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}
