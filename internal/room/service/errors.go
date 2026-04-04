package service

import "errors"

var (
	ErrInvalidRoomName                = errors.New("invalid room name")
	ErrRoomNotFound                   = errors.New("room not found")
	ErrRoomFull                       = errors.New("room is full")
	ErrForbidden                      = errors.New("forbidden")
	ErrInvalidGameType                = errors.New("invalid game type")
	ErrInvalidMaxPlayers              = errors.New("invalid max players")
	ErrNotEnoughPlayers               = errors.New("not enough players")
	ErrRoomNotReady                   = errors.New("room is not ready to start")
	ErrRoomNotJoinable                = errors.New("room is not joinable")
	ErrRoomSettingsLocked             = errors.New("room settings are locked")
	ErrMaxPlayersLessThanParticipants = errors.New("max players is less than current participants count")
	ErrInvalidRoomSettings            = errors.New("invalid room settings")
)
