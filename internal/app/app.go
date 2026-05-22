package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	carcassonneHTTP "github.com/webmafia/tumladan/internal/game/carcassonne/delivery/http"
	carcassonneService "github.com/webmafia/tumladan/internal/game/carcassonne/service"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	guestHTTP "github.com/webmafia/tumladan/internal/guest/delivery/http"
	guestPostgres "github.com/webmafia/tumladan/internal/guest/repository/postgres"
	guestService "github.com/webmafia/tumladan/internal/guest/service"
	matchPostgres "github.com/webmafia/tumladan/internal/match/repository/postgres"
	matchService "github.com/webmafia/tumladan/internal/match/service"
	"github.com/webmafia/tumladan/internal/middleware"
	roomHTTP "github.com/webmafia/tumladan/internal/room/delivery/http"
	roomPostgres "github.com/webmafia/tumladan/internal/room/repository/postgres"
	roomService "github.com/webmafia/tumladan/internal/room/service"
	"github.com/webmafia/tumladan/internal/router"
	userHTTP "github.com/webmafia/tumladan/internal/user/delivery/http"
	userPostgres "github.com/webmafia/tumladan/internal/user/repository/postgres"
	userStorage "github.com/webmafia/tumladan/internal/user/repository/storage"
	userService "github.com/webmafia/tumladan/internal/user/service"
	internalws "github.com/webmafia/tumladan/internal/ws"
	wsticketHTTP "github.com/webmafia/tumladan/internal/ws_ticket/delivery/http"
	wsticketService "github.com/webmafia/tumladan/internal/ws_ticket/service"
	wsticketStore "github.com/webmafia/tumladan/internal/ws_ticket/store"
	"github.com/webmafia/tumladan/pkg/csrfmanager"
	jwtprovider "github.com/webmafia/tumladan/pkg/jwt"
	"github.com/webmafia/tumladan/pkg/logger"
	miniostore "github.com/webmafia/tumladan/pkg/minio"
	"github.com/webmafia/tumladan/pkg/postgres"
)

type App struct {
	cfg     *Config
	logger  logger.Logger
	server  *http.Server
	db      *sql.DB
	ctx     context.Context
	stop    context.CancelFunc
	roomSvc *roomService.Service
	ws      *internalws.Handler
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

	minioClient, err := miniostore.New(ctx, cfg.MinIO)
	if err != nil {
		stop()
		_ = db.Close()
		return nil, err
	}

	jwtProvider := jwtprovider.New(cfg.JWTSecret, cfg.JWTTTL)
	authMiddleware := middleware.NewAuth(jwtProvider)
	csrfManager := csrfmanager.NewManager(cfg.CSRFSecret, cfg.CSRFTTL)
	csrfMiddleware := middleware.NewCSRFMiddleware(csrfManager)

	carcassonneEngine, err := carcassonneService.NewEngine()
	if err != nil {
		stop()
		_ = db.Close()
		return nil, err
	}
	carcassonneHandler := carcassonneHTTP.NewHandler(carcassonneEngine)

	gamesRegistry, err := gameService.NewRegistry(carcassonneEngine)
	if err != nil {
		stop()
		_ = db.Close()
		return nil, err
	}
	gamesFacade := gameService.NewFacade(gamesRegistry)

	guestRepo := guestPostgres.New(db)
	guestSvc := guestService.New(guestRepo, jwtProvider)
	guestHandler := guestHTTP.NewHandler(guestSvc)

	userRepo := userPostgres.New(db)
	avatarStorage := userStorage.New(minioClient, cfg.MinIO.AvatarBucket)
	userSvc := userService.New(userRepo, avatarStorage, jwtProvider)
	userHandler := userHTTP.NewHandler(userSvc, csrfManager, []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
		"image/gif",
		"image/bmp",
		"image/svg",
		"image/svg+xml",
		"image/avif",
		"image/heic",
		"image/heif",
		"image/ico",
		"image/x-icon",
		"image/vnd.microsoft.icon",
	})

	roomRepo := roomPostgres.New(db)
	roomSvc := roomService.New(roomRepo, gamesFacade)
	roomHandler := roomHTTP.NewHandler(roomSvc)

	matchRepo := matchPostgres.New(db)
	matchSvc := matchService.New(matchRepo, roomSvc, gamesFacade)

	wsTicketStore := wsticketStore.NewStore(1 * time.Minute)
	wsTicketSvc := wsticketService.NewService(wsTicketStore)
	wsTicketHandler := wsticketHTTP.NewHandler(wsTicketSvc)

	wsHandler, err := internalws.NewHandler(roomSvc, matchSvc, wsTicketSvc, gamesFacade, cfg.CORS, logger)
	if err != nil {
		stop()
		_ = db.Close()
		return nil, err
	}
	if err := wsHandler.Run(); err != nil {
		stop()
		_ = db.Close()
		return nil, err
	}

	r := router.NewRouter(
		router.AppHandlers{
			GuestHandler:       guestHandler,
			UserHandler:        userHandler,
			CarcassonneHandler: carcassonneHandler,
			RoomHandler:        roomHandler,
			WSHandler:          wsHandler,
			WSTicketHandler:    wsTicketHandler,
		},
		healthHandler(logger, db),
		authMiddleware,
		csrfMiddleware,
		logger,
		cfg.CORS,
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		cfg:     cfg,
		logger:  logger,
		server:  server,
		db:      db,
		ctx:     ctx,
		stop:    stop,
		roomSvc: roomSvc,
		ws:      wsHandler,
	}, nil
}

func (a *App) Run() error {
	defer a.stop()
	defer a.db.Close()
	defer a.logger.Sync()

	a.logger.Infof("starting server env=%s addr=%s", a.cfg.AppEnv, a.cfg.HTTPAddress())

	serverErrCh := make(chan error, 1)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		close(serverErrCh)
	}()

	go a.runRoomCleanup()

	select {
	case <-a.ctx.Done():
		a.logger.Infof("shutdown signal received")
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

	if a.ws != nil {
		if err := a.ws.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}

	a.logger.Infof("server stopped")
	return nil
}

func (a *App) runRoomCleanup() {
	ticker := time.NewTicker(a.cfg.RoomCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			terminatedRoomIDs, err := a.roomSvc.CleanupStaleRooms(a.ctx, a.cfg.RoomWaitingCleanupTTL, a.cfg.RoomPlayingCleanupTTL)
			if err != nil {
				a.logger.Errorf("room cleanup failed error=%v", err)
				continue
			}

			for _, roomID := range terminatedRoomIDs {
				a.ws.NotifyMatchFinished(a.ctx, roomID)
			}
		}
	}
}

func newLogger(cfg *Config) logger.Logger {
	mode := logger.ModeProd
	if cfg.AppEnv == "local" || cfg.AppEnv == "dev" {
		mode = logger.ModeDev
	}

	log, err := logger.New(cfg.LogLevel, mode)
	if err != nil {
		log, _ = logger.New(logger.LevelInfo, logger.ModeDev)
	}

	return log
}

func healthHandler(logger logger.Logger, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			middleware.LoggerFromContextOr(ctx, logger).With("error", err).Errorf("healthcheck failed")
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
