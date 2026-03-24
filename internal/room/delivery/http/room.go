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

//TODO: add pkg/response + logger

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.service.CreateRoom(r.Context(), actor, req)
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

func (h *Handler) GetRoomByInviteCode(w http.ResponseWriter, r *http.Request) {
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
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateRoomSettings(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "id")
	if roomID == "" {
		http.Error(w, "room id is required", http.StatusBadRequest)
		return
	}

	var req roomDTO.UpdateRoomSettingsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	resp, err := h.service.UpdateRoomSettings(r.Context(), actor, roomID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoomNotFound):
			http.Error(w, "room not found", http.StatusNotFound)
		case errors.Is(err, service.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, service.ErrInvalidGameType),
			errors.Is(err, service.ErrInvalidMaxPlayers):
			http.Error(w, "invalid room settings", http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) StartRoom(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "id")
	if roomID == "" {
		http.Error(w, "room id is required", http.StatusBadRequest)
		return
	}

	resp, err := h.service.StartRoom(r.Context(), actor.ID, roomID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRoomNotFound):
			http.Error(w, "room not found", http.StatusNotFound)
		case errors.Is(err, service.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, service.ErrNotEnoughPlayers),
			errors.Is(err, service.ErrRoomNotReady):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
