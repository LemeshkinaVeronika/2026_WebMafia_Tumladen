package http

import (
	"encoding/json"
	"errors"
	"net/http"

	guestDTO "github.com/webmafia/tumladan/internal/guest/dto"
	"github.com/webmafia/tumladan/internal/guest/service"
	"github.com/webmafia/tumladan/internal/middleware"
)

//TODO:pkg response

func (h *Handler) CreateGuestSession(w http.ResponseWriter, r *http.Request) {
	const op = "guest.delivery.http.CreateGuestSession"

	var req guestDTO.CreateGuestSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.service.CreateGuestSession(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDisplayName) {
			http.Error(w, "invalid display name", http.StatusBadRequest)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		middleware.LoggerFromContext(r.Context()).Errorf("[%s]: encode response failed: %v", op, err)
	}
}
