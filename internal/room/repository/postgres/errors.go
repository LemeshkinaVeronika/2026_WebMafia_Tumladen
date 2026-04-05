package postgres

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound                       = errors.New("room not found in postgres repository")
	ErrRoomFull                       = errors.New("room is full in postgres repository")
	ErrRoomNotJoinable                = errors.New("room is not joinable in postgres repository")
	ErrMaxPlayersLessThanParticipants = errors.New("max players is less than current participants count in postgres repository")
	ErrRoomSettingsLocked             = errors.New("room settings are locked in postgres repository")
	ErrRoomNotReady                   = errors.New("room is not ready to start in postgres repository")
	ErrNotEnoughPlayers               = errors.New("not enough players in postgres repository")
	ErrActiveMatchNotFound            = errors.New("active match not found in postgres repository")
	ErrForbiddenDeleteActiveRoom      = errors.New("cannot delete active room in postgres repository")
)

func mapErrors(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	default:
		return err
	}
}
