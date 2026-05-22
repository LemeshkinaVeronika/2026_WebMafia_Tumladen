package model

import "time"

type RoomStatus string

const (
	RoomStatusWaiting RoomStatus = "waiting"
	RoomStatusPlaying RoomStatus = "playing"
	RoomStatusClosed  RoomStatus = "closed"
)

type Room struct {
	ID             string
	Name           string
	IsPrivate      bool
	InviteCode     *string
	OwnerActorID   string
	OwnerActorType ActorType
	Status         RoomStatus
	GameType       string
	MaxPlayers     int
	Settings       JSONB
	PlayersCount   int
	LastEmptyAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CurrentRoom struct {
	ID       string
	Name     string
	Status   RoomStatus
	GameType string
	MatchID  *string
}
