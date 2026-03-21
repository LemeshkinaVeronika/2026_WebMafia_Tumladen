package router

import (
	"net/http"

	guestDelivery "github.com/webmafia/tumladan/internal/guest/delivery/http"
	roomDelivery "github.com/webmafia/tumladan/internal/room/delivery/http"

	"github.com/gorilla/mux"
)

type AppHandlers struct {
	GuestHandler *guestDelivery.Handler
	RoomHandler  *roomDelivery.Handler
}

func NewRouter(handlers AppHandlers, healthHandler http.HandlerFunc) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/health", healthHandler).Methods(http.MethodGet)

	api := r.PathPrefix("/api/v1").Subrouter()
	public := api.PathPrefix("").Subrouter()

	if handlers.GuestHandler != nil {
		handlers.GuestHandler.RegisterRoutes(public)
	}

	if handlers.RoomHandler != nil {
		handlers.RoomHandler.RegisterRoutes(public)
	}

	return r
}
