package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	guestHTTP "github.com/webmafia/tumladan/internal/guest/delivery/http"
	guestPostgres "github.com/webmafia/tumladan/internal/guest/repository/postgres"
	guestService "github.com/webmafia/tumladan/internal/guest/service"
	roomHTTP "github.com/webmafia/tumladan/internal/room/delivery/http"
	roomPostgres "github.com/webmafia/tumladan/internal/room/repository/postgres"
	roomService "github.com/webmafia/tumladan/internal/room/service"
	"github.com/webmafia/tumladan/internal/router"
	"github.com/webmafia/tumladan/pkg/jwt"
	"github.com/webmafia/tumladan/pkg/postgres"
)

type App struct {
	cfg    *Config
	logger *slog.Logger
	server *http.Server
	db     *sql.DB
	ctx    context.Context
	stop   context.CancelFunc
}

func New(ctx context.Context) (*App, error) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	cfg, err := Load()
	if err != nil {
		stop()
		return nil, err
	}

	logger := newLogger(cfg)

	db, err := postgres.New(ctx, cfg.Postgres)
	if err != nil {
		stop()
		return nil, err
	}

	jwtProvider := jwtprovider.New(cfg.JWTSecret, cfg.JWTTTL)

	guestRepo := guestPostgres.New(db)
	guestSvc := guestService.New(guestRepo, jwtProvider)
	guestHandler := guestHTTP.NewHandler(guestSvc)

	roomRepo := roomPostgres.New(db)
	roomSvc := roomService.New(roomRepo)
	roomHandler := roomHTTP.NewHandler(roomSvc)

	r := router.NewRouter(
		router.AppHandlers{
			GuestHandler: guestHandler,
			RoomHandler:  roomHandler,
		},
		healthHandler(logger, db),
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		cfg:    cfg,
		logger: logger,
		server: server,
		db:     db,
		ctx:    ctx,
		stop:   stop,
	}, nil
}

func (a *App) Run() error {
	defer a.stop()
	defer a.db.Close()

	a.logger.Info("starting server",
		"env", a.cfg.AppEnv,
		"addr", a.cfg.HTTPAddress(),
	)

	serverErrCh := make(chan error, 1)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		close(serverErrCh)
	}()

	select {
	case <-a.ctx.Done():
		a.logger.Info("shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	a.logger.Info("server stopped")
	return nil
}

func newLogger(cfg *Config) *slog.Logger {
	var level slog.Level

	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}

func healthHandler(logger *slog.Logger, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			logger.Error("healthcheck failed", "error", err)
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
