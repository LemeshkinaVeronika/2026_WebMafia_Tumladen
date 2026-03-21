package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.MethodFunc(http.MethodPost, "/guest-sessions", h.CreateGuestSession)
}
