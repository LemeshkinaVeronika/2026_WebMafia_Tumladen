package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/webmafia/tumladan/internal/middleware"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	"github.com/webmafia/tumladan/internal/room/service"
)

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	const op = "room.delivery.http.CreateRoom"

	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req roomDTO.CreateRoomRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	resp, err := h.service.CreateRoom(r.Context(), roomDTO.CreateRoomServiceRequest{
		Actor: roomDTO.ActorRequest{
			ID:          actor.ID,
			Type:        string(actor.Type),
			DisplayName: actor.DisplayName,
		},
		Name:      req.Name,
		IsPrivate: req.IsPrivate,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRoomName):
			http.Error(w, "invalid room name", http.StatusBadRequest)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		middleware.LoggerFromContext(r.Context()).Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) ListPublicRooms(w http.ResponseWriter, r *http.Request) {
	const op = "room.delivery.http.ListPublicRooms"

	resp, err := h.service.ListPublicRooms(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		middleware.LoggerFromContext(r.Context()).Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) GetRoomByInviteCode(w http.ResponseWriter, r *http.Request) {
	const op = "room.delivery.http.GetRoomByInviteCode"

	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "invite code is required", http.StatusBadRequest)
		return
	}

	resp, err := h.service.GetRoomByInviteCode(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoomNotFound):
			http.Error(w, "room not found", http.StatusNotFound)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		middleware.LoggerFromContext(r.Context()).Errorf("[%s]: encode response failed: %v", op, err)
	}
}
