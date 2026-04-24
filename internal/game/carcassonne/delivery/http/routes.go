package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.MethodFunc(http.MethodGet, "/games/carcassonne/tiles", h.GetTileCatalog)
}
