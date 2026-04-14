package ws

import (
	"context"
	"sync"

	"github.com/webmafia/tumladan/internal/model"
)

type authSessionContextKey struct{}
type ticketHandshakeContextKey struct{}

type ticketHandshake struct {
	mu       sync.Mutex
	ticket   string
	finished bool
}

func newTicketHandshake(ticket string) *ticketHandshake {
	return &ticketHandshake{ticket: ticket}
}

func (h *ticketHandshake) Ticket() string {
	if h == nil {
		return ""
	}
	return h.ticket
}

func (h *ticketHandshake) Commit(commit func(string) error) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return nil
	}

	if err := commit(h.ticket); err != nil {
		return err
	}

	h.finished = true
	return nil
}

func (h *ticketHandshake) Release(release func(string)) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return
	}

	release(h.ticket)
	h.finished = true
}

func authSessionFromContext(ctx context.Context) (model.AuthSession, bool) {
	session, ok := ctx.Value(authSessionContextKey{}).(model.AuthSession)
	return session, ok
}

func ticketHandshakeFromContext(ctx context.Context) (*ticketHandshake, bool) {
	handshake, ok := ctx.Value(ticketHandshakeContextKey{}).(*ticketHandshake)
	return handshake, ok
}
