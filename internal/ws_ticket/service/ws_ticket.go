package service

import (
	"github.com/webmafia/tumladan/internal/model"
	wsticket "github.com/webmafia/tumladan/internal/ws_ticket/store"
)

type Store interface {
	Create(session model.AuthSession) (string, error)
	Reserve(ticket string) (model.AuthSession, error)
	Resolve(ticket string) (model.AuthSession, error)
	Release(ticket string)
	Commit(ticket string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

var _ Store = (*wsticket.Store)(nil)
