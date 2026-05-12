package middleware

import (
	"net/http"
)

const CSRFHeader = "X-CSRF-Token"

type CSRFManager interface {
	Check(userID, sessionID, clientToken string) (bool, error)
}

type CSRF struct {
	csrfManager CSRFManager
}

func NewCSRFMiddleware(csrfManager CSRFManager) *CSRF {
	return &CSRF{csrfManager: csrfManager}
}

func (m *CSRF) CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const op = "middleware.CSRF"
		log := LoggerFromContext(r.Context())

		session, ok := AuthSessionFromContext(r.Context())
		if !ok {
			log.Errorf("[%s]: missing auth session", op)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		csrfToken := r.Header.Get(CSRFHeader)
		if csrfToken == "" {
			log.Warnf("[%s]: missing CSRF token in header", op)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		isValid, err := m.csrfManager.Check(session.Actor.ID, session.SessionID, csrfToken)
		if err != nil || !isValid {
			log.Warnf("[%s]: invalid CSRF token: %v", op, err)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
