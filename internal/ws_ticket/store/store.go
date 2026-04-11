package store

import (
	"sync"
	"time"
)

type Ticket struct {
	Value       string
	SessionID   string
	ActorID     string
	ActorType   string
	DisplayName string
	ExpiresAt   time.Time
	Reserved    bool
}

type Store struct {
	mu      sync.Mutex
	tickets map[string]Ticket
	ttl     time.Duration
}

func NewStore(ttl time.Duration) *Store {
	return &Store{
		tickets: make(map[string]Ticket),
		ttl:     ttl,
	}
}
