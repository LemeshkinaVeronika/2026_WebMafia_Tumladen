package service

import (
	"github.com/webmafia/tumladan/internal/model"
)

func (s *Service) Create(session model.AuthSession) (string, error) {
	return s.store.Create(session)
}

func (s *Service) Reserve(ticket string) (model.AuthSession, error) {
	return s.store.Reserve(ticket)
}

func (s *Service) Resolve(ticket string) (model.AuthSession, error) {
	return s.store.Resolve(ticket)
}

func (s *Service) Release(ticket string) {
	s.store.Release(ticket)
}

func (s *Service) Commit(ticket string) error {
	return s.store.Commit(ticket)
}
