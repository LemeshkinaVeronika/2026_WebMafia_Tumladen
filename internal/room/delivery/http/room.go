package http

import (
	"encoding/json"
	"errors"
	"net/http"

	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	"github.com/webmafia/tumladan/internal/room/service"
)

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req roomDTO.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.service.CreateRoom(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRoomName) {
			http.Error(w, "invalid room name", http.StatusBadRequest)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListPublicRooms(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ListPublicRooms(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
