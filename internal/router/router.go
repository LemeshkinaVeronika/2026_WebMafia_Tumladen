package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	guestDelivery "github.com/webmafia/tumladan/internal/guest/delivery/http"
	"github.com/webmafia/tumladan/internal/middleware"
	roomDelivery "github.com/webmafia/tumladan/internal/room/delivery/http"
	internalws "github.com/webmafia/tumladan/internal/ws"
)

type AppHandlers struct {
	GuestHandler *guestDelivery.Handler
	RoomHandler  *roomDelivery.Handler
	WSHandler    *internalws.Handler
	cors         middleware.CORSConfig
}

func NewRouter(handlers AppHandlers, healthHandler http.HandlerFunc, auth *middleware.Auth, cors middleware.CORSConfig) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.CORS(cors))

	r.Get("/health", healthHandler)

	if handlers.WSHandler != nil {
		r.Handle("/ws", handlers.WSHandler)
	}

	r.Route("/api/v1", func(api chi.Router) {
		if handlers.GuestHandler != nil {
			handlers.GuestHandler.RegisterRoutes(api)
		}

		if handlers.RoomHandler != nil {
			handlers.RoomHandler.RegisterPublicRoutes(api)
		}

		if auth != nil && handlers.RoomHandler != nil {
			api.Group(func(protected chi.Router) {
				protected.Use(auth.AuthMiddleware)
				handlers.RoomHandler.RegisterProtectedRoutes(protected)
			})
		}
	})

	return r
}
