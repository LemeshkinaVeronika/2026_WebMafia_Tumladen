package http

import (
	"encoding/json"
	"github.com/webmafia/tumladan/internal/middleware"
	wsticketdto "github.com/webmafia/tumladan/internal/ws_ticket/dto"
	"net/http"
)

func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.AuthSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ticket, err := h.service.Create(session)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(wsticketdto.CreateTicketResponse{
		Ticket: ticket,
	})
}
