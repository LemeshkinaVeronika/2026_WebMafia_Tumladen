package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	carcassonneDelivery "github.com/webmafia/tumladan/internal/game/carcassonne/delivery/http"
	guestDelivery "github.com/webmafia/tumladan/internal/guest/delivery/http"
	"github.com/webmafia/tumladan/internal/middleware"
	roomDelivery "github.com/webmafia/tumladan/internal/room/delivery/http"
	internalws "github.com/webmafia/tumladan/internal/ws"
	wsticketDelivery "github.com/webmafia/tumladan/internal/ws_ticket/delivery/http"
	"github.com/webmafia/tumladan/pkg/logger"
)

type AppHandlers struct {
	GuestHandler       *guestDelivery.Handler
	CarcassonneHandler *carcassonneDelivery.Handler
	RoomHandler        *roomDelivery.Handler
	WSHandler          *internalws.Handler
	cors               middleware.CORSConfig
	WSTicketHandler    *wsticketDelivery.Handler
}

func NewRouter(handlers AppHandlers, healthHandler http.HandlerFunc, auth *middleware.Auth, log logger.Logger, cors middleware.CORSConfig) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.CORS(cors))
	if log != nil {
		r.Use(middleware.RequestLoggerMiddleware(log))
	}

	r.Get("/health", healthHandler)

	if handlers.WSHandler != nil {
		handlers.WSHandler.RegisterRoutes(r)
	}

	r.Route("/api/v1", func(api chi.Router) {
		if handlers.GuestHandler != nil {
			handlers.GuestHandler.RegisterRoutes(api)
		}

		if handlers.RoomHandler != nil {
			handlers.RoomHandler.RegisterPublicRoutes(api)
		}

		if handlers.CarcassonneHandler != nil {
			handlers.CarcassonneHandler.RegisterPublicRoutes(api)
		}

		if auth != nil && handlers.RoomHandler != nil {
			api.Group(func(protected chi.Router) {
				protected.Use(auth.AuthMiddleware)
				handlers.RoomHandler.RegisterProtectedRoutes(protected)
				if handlers.WSTicketHandler != nil {
					handlers.WSTicketHandler.RegisterProtectedRoutes(protected)
				}
			})
		}
	})

	return r
}
