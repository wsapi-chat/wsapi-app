package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// Server wraps the HTTP server with graceful shutdown.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
	logger *slog.Logger
}

// New creates a new Server. The caller is responsible for setting up routes
// on the returned chi.Mux before calling Run.
func New(cfg *config.Config, logger *slog.Logger) *Server {
	return &Server{
		cfg:    cfg,
		router: chi.NewMux(),
		logger: logger,
	}
}

// Router returns the underlying chi.Mux so routes can be registered.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (s *Server) Run(ctx context.Context) error {
	writeTimeout := s.cfg.Server.WriteTimeoutDuration()
	if writeTimeout <= whatsapp.SendAckTimeout {
		// A stalled send holds the handler for the full send-ack timeout. If the
		// write deadline lands first, Go kills the connection before the error
		// response can be written and the caller gets a reset with no status,
		// no body, and nothing in the logs naming the cause.
		s.logger.Warn("server.writeTimeout is not longer than the WhatsApp send-ack timeout; "+
			"stalled sends will be reported to callers as a dropped connection instead of an error",
			"writeTimeout", writeTimeout,
			"sendAckTimeout", whatsapp.SendAckTimeout,
			"suggested", whatsapp.SendAckTimeout+15*time.Second)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  s.cfg.Server.ReadTimeoutDuration(),
		WriteTimeout: writeTimeout,
	}

	// Channel to listen for errors from ListenAndServe
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting HTTP server", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-quit:
		s.logger.Info("shutdown signal received", "signal", sig.String())
	case <-ctx.Done():
		s.logger.Info("context cancelled")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeoutDuration())
	defer cancel()

	s.logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.logger.Info("server stopped gracefully")
	return nil
}
