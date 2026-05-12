package http

import (
	"net/http"

	"github.com/webmafia/tumladan/internal/middleware"
	"github.com/webmafia/tumladan/internal/model"
)

type csrfResponse struct {
	Token string `json:"csrf_token"`
}

func (h *Handler) GetCSRFToken(w http.ResponseWriter, r *http.Request) {
	const op = "handler.user.GetCSRFToken"
	log := middleware.LoggerFromContext(r.Context())

	session, ok := middleware.AuthSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if session.Actor.Type != model.ActorTypeUser || session.Actor.ID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := h.csrfManager.Generate(session.Actor.ID, session.SessionID)
	if err != nil {
		log.Errorf("[%s]: failed to generate csrf token: %v", op, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := writeJSON(w, http.StatusOK, csrfResponse{Token: token}); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}
