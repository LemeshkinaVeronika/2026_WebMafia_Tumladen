package http

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/guest-sessions", h.CreateGuestSession).Methods(http.MethodPost, http.MethodOptions)
}
