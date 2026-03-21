package model

import "time"

type RoomStatus string

const (
	RoomStatusWaiting RoomStatus = "waiting"
	RoomStatusPlaying RoomStatus = "playing"
	RoomStatusClosed  RoomStatus = "closed"
)

type Room struct {
	ID           string
	Name         string
	IsPrivate    bool
	InviteCode   *string
	OwnerActorID string
	Status       RoomStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
