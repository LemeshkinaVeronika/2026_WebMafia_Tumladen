package store

import (
	"sync"
	"time"
)

type Ticket struct {
	Value       string    `json:"value"`
	SessionID   string    `json:"sessionId"`
	ActorID     string    `json:"actorId"`
	ActorType   string    `json:"actorType"`
	DisplayName string    `json:"displayName"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Reserved    bool      `json:"reserved"`
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
