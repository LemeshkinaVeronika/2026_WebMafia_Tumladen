package service

import "errors"

var (
	ErrInvalidRoomName = errors.New("invalid room name")
	ErrRoomNotFound    = errors.New("room not found")
)
