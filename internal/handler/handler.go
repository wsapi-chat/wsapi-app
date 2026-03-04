package handler

import (
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
func (h *Handler) ServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, whatsapp.ErrNotFound):
		h.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, whatsapp.ErrUpstream):
		h.Error(w, err.Error(), http.StatusBadGateway)
	default:
		h.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// Error sends a JSON error response.
func (h *Handler) Error(w http.ResponseWriter, detail string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"detail": detail,
	})
}

// Health is the health-check handler.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
