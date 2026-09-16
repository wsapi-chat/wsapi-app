package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wsapi-chat/wsapi-app/internal/instance"
	"github.com/wsapi-chat/wsapi-app/internal/server/middleware"
	"github.com/wsapi-chat/wsapi-app/internal/validate"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// statusClientClosedRequest is nginx's de-facto convention for "client gave
// up before we could respond." Not in net/http, but widely understood.
const statusClientClosedRequest = 499

// Handler provides shared helpers for all HTTP handlers.
type Handler struct {
	Logger *slog.Logger
}

// Instance extracts the resolved Instance from the request context.
// The RequireService middleware guarantees the instance and its Service are non-nil.
func (h *Handler) Instance(r *http.Request) *instance.Instance {
	return r.Context().Value(middleware.InstanceKey).(*instance.Instance)
}

// Decode reads and validates a JSON request body into target.
// Returns false and writes an error response if decoding or validation fails.
func (h *Handler) Decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		h.Error(w, fmt.Sprintf("invalid request body: %s", err), http.StatusBadRequest)
		return false
	}
	if err := validate.Struct(target); err != nil {
		h.Error(w, fmt.Sprintf("validation error: %s", err), http.StatusBadRequest)
		return false
	}
	return true
}

// JSON writes a JSON response with the given status code.
func (h *Handler) JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.Logger.Error("failed to encode JSON response", "error", err)
	}
}

// Created sends a 201 response with {"id": id}.
func (h *Handler) Created(w http.ResponseWriter, id string) {
	h.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

// NoContent sends a 204 No Content response.
func (h *Handler) NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ServiceError maps a service-layer error to the appropriate HTTP status code.
//
// Every branch logs. The response body used to be the only record that a
// request had failed, so if the write deadline expired before the handler
// replied (see ServerConfig.WriteTimeoutDuration) the failure left no trace.
func (h *Handler) ServiceError(w http.ResponseWriter, err error) {
	status := h.serviceStatus(err)

	// Debug only because the detail is uninformative ("context canceled") and
	// the request-logging middleware already records the 499 and its duration
	// at Info. A cancellation is not benign: the caller gave up at its own,
	// shorter timeout on a request we were still working on.
	level := slog.LevelWarn
	if status == statusClientClosedRequest {
		level = slog.LevelDebug
	}
	h.log().Log(context.Background(), level, "request failed", "status", status, "error", err)

	if status == statusClientClosedRequest {
		// Client disconnected before we could respond. The body likely
		// won't land, but we still emit a status so any intermediary
		// proxy doesn't see a hung connection.
		h.Error(w, "request canceled", status)
		return
	}
	h.Error(w, err.Error(), status)
}

// serviceStatus classifies a service-layer error. Split out from ServiceError so
// the mapping can be tested without an http.ResponseWriter.
func (h *Handler) serviceStatus(err error) int {
	switch {
	case errors.Is(err, whatsapp.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, whatsapp.ErrUpstream):
		return http.StatusBadGateway
	case errors.Is(err, whatsapp.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, whatsapp.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case whatsapp.IsUpstreamTimeout(err):
		// WhatsApp didn't answer in time — either no ack for the message or a
		// stalled info query. Transient and retryable, so it must not land in
		// the default 400 branch: a 400 tells the caller their request was
		// malformed and stops well-behaved clients from retrying.
		return http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		return statusClientClosedRequest
	default:
		return http.StatusBadRequest
	}
}

// log returns the handler's logger, falling back to the default so a
// zero-value Handler (as used in some tests) doesn't panic.
func (h *Handler) log() *slog.Logger {
	if h.Logger == nil {
		return slog.Default()
	}
	return h.Logger
}

// Error sends a JSON error response.
func (h *Handler) Error(w http.ResponseWriter, detail string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"detail": detail,
	})
}

// Health is the health-check handler.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
