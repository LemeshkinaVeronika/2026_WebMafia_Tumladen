package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/webmafia/tumladan/internal/model"
)

func (s *Store) Create(session model.AuthSession) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	value, err := generateToken(32)
	if err != nil {
		return "", err
	}

	ticket := Ticket{
		Value:       value,
		SessionID:   session.SessionID,
		ActorID:     session.Actor.ID,
		ActorType:   string(session.Actor.Type),
		DisplayName: session.Actor.DisplayName,
		ExpiresAt:   time.Now().UTC().Add(s.ttl),
	}

	s.tickets[value] = ticket

	return value, nil
}

func (s *Store) Reserve(value string) (model.AuthSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	ticket, ok := s.tickets[value]
	if !ok {
		return model.AuthSession{}, ErrTicketNotFound
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		delete(s.tickets, value)
		return model.AuthSession{}, ErrTicketExpired
	}

	if ticket.Reserved {
		return model.AuthSession{}, ErrTicketInUse
	}

	ticket.Reserved = true
	s.tickets[value] = ticket

	return model.AuthSession{
		SessionID: ticket.SessionID,
		Actor: model.Actor{
			ID:          ticket.ActorID,
			Type:        model.ActorType(ticket.ActorType),
			DisplayName: ticket.DisplayName,
		},
	}, nil
}

func (s *Store) Resolve(value string) (model.AuthSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	ticket, ok := s.tickets[value]
	if !ok {
		return model.AuthSession{}, ErrTicketNotFound
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		delete(s.tickets, value)
		return model.AuthSession{}, ErrTicketExpired
	}

	return model.AuthSession{
		SessionID: ticket.SessionID,
		Actor: model.Actor{
			ID:          ticket.ActorID,
			Type:        model.ActorType(ticket.ActorType),
			DisplayName: ticket.DisplayName,
		},
	}, nil
}

func (s *Store) Release(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[value]
	if !ok {
		return
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		delete(s.tickets, value)
		return
	}

	ticket.Reserved = false
	s.tickets[value] = ticket
}

func (s *Store) Commit(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[value]
	if !ok {
		return ErrTicketNotFound
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		delete(s.tickets, value)
		return ErrTicketExpired
	}

	if !ticket.Reserved {
		return ErrTicketNotReserved
	}

	delete(s.tickets, value)
	return nil
}

func (s *Store) cleanupExpiredLocked() {
	now := time.Now().UTC()
	for key, ticket := range s.tickets {
		if now.After(ticket.ExpiresAt) {
			delete(s.tickets, key)
		}
	}
}

func generateToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
