package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/webmafia/tumladan/internal/middleware"
)

func TestHealthProbeRoutes(t *testing.T) {
	readiness := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	router := NewRouter(AppHandlers{}, readiness, nil, nil, nil, middleware.CORSConfig{})

	for _, test := range []struct {
		path   string
		status int
	}{
		{path: "/live", status: http.StatusOK},
		{path: "/ready", status: http.StatusServiceUnavailable},
		{path: "/health", status: http.StatusServiceUnavailable},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("GET %s status = %d, want %d", test.path, recorder.Code, test.status)
		}
	}
}
