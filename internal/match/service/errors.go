package service

import "errors"

var (
	ErrMatchNotFound      = errors.New("match not found")
	ErrMatchNotActive     = errors.New("match is not active")
	ErrInvalidMatchAction = errors.New("invalid match action")
	ErrNotYourTurn        = errors.New("not your turn")
	ErrMatchStateConflict = errors.New("match state does not match action preconditions")
	ErrActionIDConflict   = errors.New("action id was already used for a different request")
)
