package store

import "errors"

var (
	ErrTicketNotFound    = errors.New("ws ticket not found")
	ErrTicketExpired     = errors.New("ws ticket expired")
	ErrTicketInUse       = errors.New("ws ticket already in use")
	ErrTicketNotReserved = errors.New("ws ticket not reserved")
)
