package service

import "errors"

var (
	ErrMatchNotFound      = errors.New("match not found")
	ErrMatchNotActive     = errors.New("match is not active")
	ErrInvalidMatchAction = errors.New("invalid match action")
	ErrNotYourTurn        = errors.New("not your turn")
)
