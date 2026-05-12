package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/webmafia/tumladan/internal/middleware"
)

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.MethodFunc(http.MethodPost, "/users/register", h.Register)
	r.MethodFunc(http.MethodPost, "/users/login", h.Login)
	r.MethodFunc(http.MethodGet, "/users/{id}", h.GetPublicProfile)
}

func (h *Handler) RegisterProtectedRoutes(r chi.Router, csrf *middleware.CSRF) {
	r.MethodFunc(http.MethodGet, "/users/csrf-token", h.GetCSRFToken)
	r.MethodFunc(http.MethodGet, "/users/me", h.GetProfile)

	r.Group(func(csrfProtected chi.Router) {
		if csrf != nil {
			csrfProtected.Use(csrf.CSRFMiddleware)
		}

		csrfProtected.MethodFunc(http.MethodPost, "/users/logout", h.Logout)
		csrfProtected.MethodFunc(http.MethodPost, "/users/avatar", h.UploadAvatar)
		csrfProtected.MethodFunc(http.MethodDelete, "/users/avatar", h.DeleteAvatar)
		csrfProtected.MethodFunc(http.MethodPut, "/users/profile", h.UpdateProfile)
	})
}
