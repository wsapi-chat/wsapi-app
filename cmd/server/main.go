package main

import (
	"context"
	"database/sql"
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
	"github.com/wsapi-chat/wsapi-app/internal/validate"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
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

	// Open single shared DB connection pool
	db, err := openDB(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close() //nolint:errcheck

	// Initialize whatsmeow container (device sessions, uses same driver/dsn)
	waContainer, err := whatsapp.OpenContainer(context.Background(), cfg.Database.Driver, cfg.Database.DSN, waLogger)
	if err != nil {
		logger.Error("failed to open whatsmeow store", "error", err)
		os.Exit(1)
	}

	// Run migrations for WSAPI custom tables
	if err := whatsapp.MigrateCustomTables(db, cfg.Database.Driver); err != nil {
		logger.Error("failed to migrate custom tables", "error", err)
		os.Exit(1)
	}

	// Create stores (all share the same pool)
	chatStore := whatsapp.NewChatStore(db, cfg.Database.Driver)
	contactStore := whatsapp.NewContactStore(db, cfg.Database.Driver)
	historySyncStore := whatsapp.NewHistorySyncStore(db, cfg.Database.Driver)

	var instanceStore *whatsapp.InstanceStore
	if cfg.InstanceMode != "single" {
		instanceStore = whatsapp.NewInstanceStore(db, cfg.Database.Driver)
	}

	// Initialize publisher factory (used per instance)
	pubFactory := publisher.NewFactory(cfg, logger)
	defer pubFactory.Close() //nolint:errcheck

	// Initialize instance manager
	mgr := instance.NewManager(instanceStore, waContainer, chatStore, contactStore, historySyncStore, cfg, pubFactory, logger, waLogger)
	defer mgr.Shutdown()

	// Provision instances based on mode
	if cfg.InstanceMode == "single" {
		if err := mgr.EnsureSingleInstance(context.Background()); err != nil {
			logger.Error("failed to provision single instance", "error", err)
			os.Exit(1)
		}
	} else {
		if err := mgr.RestoreInstances(context.Background()); err != nil {
			logger.Error("failed to restore instances", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("instance mode", "mode", cfg.InstanceMode)

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

	// Admin routes (multi mode only)
	if cfg.InstanceMode == "multi" {
		r.Route("/admin/instances", func(r chi.Router) {
			r.Use(middleware.AdminAuth(cfg.Auth.AdminAPIKey))
			ih := handler.NewInstanceHandler(mgr, cfg, logger)
			ih.RegisterRoutes(r)
		})
	}

	// Instance auth middleware: single mode resolves "default" automatically;
	// multi mode requires X-Instance-Id header.
	var instanceAuth func(http.Handler) http.Handler
	if cfg.InstanceMode == "single" {
		instanceAuth = middleware.SingleInstanceAuth(mgr, instance.SingleInstanceID)
	} else {
		instanceAuth = middleware.InstanceAuth(mgr)
	}

	// All other API routes
	r.Route("/", func(r chi.Router) {
		r.Use(instanceAuth)
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

// openDB opens a database connection pool and applies driver-specific pragmas.
func openDB(driver, dsn string) (*sql.DB, error) {
	dsn = applySQLitePragmas(driver, dsn)

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// applySQLitePragmas appends SQLite pragmas (foreign_keys, journal_mode=WAL,
// busy_timeout) to the DSN if missing. WAL mode and busy_timeout are essential
// because whatsmeow and WSAPI use separate connection pools against the same file.
func applySQLitePragmas(driver, dsn string) string {
	if driver != "sqlite" {
		return dsn
	}
	if !strings.Contains(dsn, "foreign_keys") {
		dsn = appendPragma(dsn, "_pragma=foreign_keys(1)")
	}
	if !strings.Contains(dsn, "journal_mode") {
		dsn = appendPragma(dsn, "_pragma=journal_mode(WAL)")
	}
	if !strings.Contains(dsn, "busy_timeout") {
		dsn = appendPragma(dsn, "_pragma=busy_timeout(5000)")
	}
	return dsn
}

func appendPragma(dsn, pragma string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pragma
	}
	return dsn + "?" + pragma
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
