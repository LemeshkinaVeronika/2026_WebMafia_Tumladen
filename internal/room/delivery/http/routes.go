package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.MethodFunc(http.MethodGet, "/rooms/public", h.ListPublicRooms)
	r.MethodFunc(http.MethodGet, "/rooms/invite/{code}", h.GetRoomByInviteCode)
}

func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.MethodFunc(http.MethodPost, "/rooms", h.CreateRoom)
	r.MethodFunc(http.MethodPatch, "/rooms/{id}/settings", h.UpdateRoomSettings)
	r.MethodFunc(http.MethodPost, "/rooms/{id}/start", h.StartRoom)
}
