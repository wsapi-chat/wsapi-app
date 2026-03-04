package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/handler"
	"github.com/wsapi-chat/wsapi-app/internal/httputil"
	"github.com/wsapi-chat/wsapi-app/internal/instance"
	"github.com/wsapi-chat/wsapi-app/internal/logging"
	"github.com/wsapi-chat/wsapi-app/internal/publisher"
	"github.com/wsapi-chat/wsapi-app/internal/server"
	"github.com/wsapi-chat/wsapi-app/internal/server/middleware"
	"github.com/wsapi-chat/wsapi-app/internal/store"
	"github.com/wsapi-chat/wsapi-app/internal/validate"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize HTTP proxy (if configured)
	if err := httputil.Init(cfg.HTTPProxy); err != nil {
		slog.Error("failed to initialize HTTP proxy", "error", err)
		os.Exit(1)
	}

	// Set up structured loggers
	logger := setupLogger(cfg)
	waLogger := setupWALogger(cfg)

	if cfg.HTTPProxy != "" {
		logger.Info("HTTP proxy configured", "proxy", cfg.HTTPProxy)
	}

	// Initialize validator
	validate.Init()

	// Initialize WSAPI store (instance persistence)
	st, err := store.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer st.Close() //nolint:errcheck

	// Initialize whatsmeow container (device sessions)
	waContainer, err := whatsapp.OpenContainer(context.Background(), cfg.Whatsmeow.Driver, cfg.Whatsmeow.DSN, waLogger)
	if err != nil {
		logger.Error("failed to open whatsmeow store", "error", err)
		os.Exit(1)
	}

	// Run migrations for WSAPI custom tables in whatsmeow DB
	if err := whatsapp.MigrateCustomTables(cfg.Whatsmeow.Driver, cfg.Whatsmeow.DSN); err != nil {
		logger.Error("failed to migrate whatsmeow custom tables", "error", err)
		os.Exit(1)
	}

	// Initialize chat store (wsapi_chats table in whatsmeow DB)
	chatStore, err := whatsapp.OpenChatStore(cfg.Whatsmeow.Driver, cfg.Whatsmeow.DSN)
	if err != nil {
		logger.Error("failed to open chat store", "error", err)
		os.Exit(1)
	}
	defer chatStore.Close() //nolint:errcheck

	// Initialize contact store (wsapi_contacts table in whatsmeow DB)
	contactStore, err := whatsapp.OpenContactStore(cfg.Whatsmeow.Driver, cfg.Whatsmeow.DSN)
	if err != nil {
		logger.Error("failed to open contact store", "error", err)
		os.Exit(1)
	}
	defer contactStore.Close() //nolint:errcheck

	// Initialize history sync store (wsapi_history_sync_messages table in whatsmeow DB)
	historySyncStore, err := whatsapp.OpenHistorySyncStore(cfg.Whatsmeow.Driver, cfg.Whatsmeow.DSN)
	if err != nil {
		logger.Error("failed to open history sync store", "error", err)
		os.Exit(1)
	}
	defer historySyncStore.Close() //nolint:errcheck

	// Initialize publisher factory (used per instance)
	pubFactory := publisher.NewFactory(cfg, logger)

	// Initialize instance manager
	mgr := instance.NewManager(st, waContainer, chatStore, contactStore, historySyncStore, cfg, pubFactory, logger, waLogger)

	// Restore persisted instances
	if err := mgr.RestoreInstances(context.Background()); err != nil {
		logger.Error("failed to restore instances", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	srv := server.New(cfg, logger)
	r := srv.Router()

	// Custom error handlers for unmatched routes
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": http.StatusNotFound,
			"detail": "not found",
		})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": http.StatusMethodNotAllowed,
			"detail": "method not allowed",
		})
	})

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))

	// Health endpoint (no auth)
	r.Get("/health", handler.Health)

	// Instance management routes (admin auth, no X-Instance-Id)
	r.Route("/admin/instances", func(r chi.Router) {
		r.Use(middleware.AdminAuth(cfg.Auth.AdminAPIKey))
		ih := handler.NewInstanceHandler(mgr, cfg, logger)
		ih.RegisterRoutes(r)
	})

	// All other API routes (instance auth)
	r.Route("/", func(r chi.Router) {
		r.Use(middleware.InstanceAuth(mgr))
		r.Use(middleware.RequireService)

		// Session routes do not require a paired device.
		sessionH := handler.NewSessionHandler(logger, mgr)
		sessionH.RegisterRoutes(r)

		// All remaining routes require a paired device.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePaired)

			messageH := handler.NewMessageHandler(logger)
			messageH.RegisterRoutes(r)

			groupH := handler.NewGroupHandler(logger)
			groupH.RegisterRoutes(r)

			communityH := handler.NewCommunityHandler(logger)
			communityH.RegisterRoutes(r)

			contactH := handler.NewContactHandler(logger)
			contactH.RegisterRoutes(r)

			chatH := handler.NewChatHandler(logger)
			chatH.RegisterRoutes(r)

			userH := handler.NewUserHandler(logger)
			userH.RegisterRoutes(r)

			mediaH := handler.NewMediaHandler(logger)
			mediaH.RegisterRoutes(r)

			callH := handler.NewCallHandler(logger)
			callH.RegisterRoutes(r)

			newsletterH := handler.NewNewsletterHandler(logger)
			newsletterH.RegisterRoutes(r)

			statusH := handler.NewStatusHandler(logger)
			statusH.RegisterRoutes(r)
		})
	})

	// Run server (blocks until shutdown)
	if err := srv.Run(context.Background()); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLogLevel(cfg.Logging.Level)}

	var h slog.Handler
	if strings.ToLower(cfg.Logging.Format) == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	if cfg.Logging.RedactPII {
		h = logging.NewRedactHandler(h, logging.DefaultDeepRedactKeys, logging.DefaultSensitiveFields)
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

func setupWALogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLogLevel(cfg.Whatsmeow.LogLevel)}

	var h slog.Handler
	if strings.ToLower(cfg.Logging.Format) == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(h).With("source", "whatsmeow")
}
