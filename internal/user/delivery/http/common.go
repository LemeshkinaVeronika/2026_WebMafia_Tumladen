package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/webmafia/tumladan/internal/user/service"
)

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		http.Error(w, "validation error", http.StatusBadRequest)
	case errors.Is(err, service.ErrConflict):
		http.Error(w, "conflict", http.StatusConflict)
	case errors.Is(err, service.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(body)
}
