package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/config"
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
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  s.cfg.Server.ReadTimeoutDuration(),
		WriteTimeout: s.cfg.Server.WriteTimeoutDuration(),
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
