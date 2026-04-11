package service

import wsticket "github.com/webmafia/tumladan/internal/ws_ticket/store"

type Service struct {
	store *wsticket.Store
}

func NewService(store *wsticket.Store) *Service {
	return &Service{store: store}
}
