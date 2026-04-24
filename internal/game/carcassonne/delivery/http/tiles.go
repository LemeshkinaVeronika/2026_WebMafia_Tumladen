package http

import (
	"net/http"

	"github.com/webmafia/tumladan/internal/middleware"
)

func (h *Handler) GetTileCatalog(w http.ResponseWriter, r *http.Request) {
	const op = "carcassonne.delivery.http.GetTileCatalog"

	resp, err := h.service.BuildTileCatalog()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(resp); err != nil {
		middleware.LoggerFromContext(r.Context()).Errorf("[%s]: write response failed: %v", op, err)
	}
}
