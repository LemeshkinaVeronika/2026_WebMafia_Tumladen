package service

import "errors"

var (
	ErrUnsupportedGameType = errors.New("unsupported game type")
	ErrInvalidRoomSettings = errors.New("invalid room settings")
	ErrInvalidRoomConfig   = errors.New("invalid room config")
	ErrInvalidMatchAction  = errors.New("invalid match action")
	ErrNotYourTurn         = errors.New("not your turn")
)
