package http

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.MethodFunc(http.MethodPost, "/ws-ticket", h.CreateTicket)
}
