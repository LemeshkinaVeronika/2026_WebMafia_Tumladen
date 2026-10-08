package postgres

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound           = errors.New("match not found in postgres repository")
	ErrStateConflict      = errors.New("match state changed in postgres repository")
	ErrActionAlreadySaved = errors.New("match action receipt already saved")
)

func mapErrors(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	default:
		return err
	}
}
