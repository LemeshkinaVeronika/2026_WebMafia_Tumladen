package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	guestDelivery "github.com/webmafia/tumladan/internal/guest/delivery/http"
	roomDelivery "github.com/webmafia/tumladan/internal/room/delivery/http"
)

type AppHandlers struct {
	GuestHandler *guestDelivery.Handler
	RoomHandler  *roomDelivery.Handler
}

func NewRouter(handlers AppHandlers, healthHandler http.HandlerFunc) *chi.Mux {
	r := chi.NewRouter()

	r.Get("/health", healthHandler)

	r.Route("/api/v1", func(api chi.Router) {
		if handlers.GuestHandler != nil {
			handlers.GuestHandler.RegisterRoutes(api)
		}

		if handlers.RoomHandler != nil {
			handlers.RoomHandler.RegisterRoutes(api)
		}
	})

	return r
}
