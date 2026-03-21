package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.MethodFunc(http.MethodPost, "/rooms", h.CreateRoom)
	router.MethodFunc(http.MethodGet, "/rooms/public", h.ListPublicRooms)
}
