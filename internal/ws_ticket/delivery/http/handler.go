package http

import (
	wsticketservice "github.com/webmafia/tumladan/internal/ws_ticket/service"
)

type Handler struct {
	service *wsticketservice.Service
}

func NewHandler(service *wsticketservice.Service) *Handler {
	return &Handler{service: service}
}
