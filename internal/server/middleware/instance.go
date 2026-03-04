package middleware

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const (
	// InstanceKey is the context key for the resolved instance.
	InstanceKey contextKey = "instance"
)

// jsonError writes a JSON error response matching the standard handler format.
func jsonError(w http.ResponseWriter, detail string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"detail": detail,
	})
}

// InstanceResolver is the minimal interface needed by the middleware to look up
// an instance and check its API key.
type InstanceResolver interface {
	GetInstance(id string) (Instance, bool)
}

// Instance is the minimal view of an instance that the middleware needs.
type Instance interface {
	GetAPIKey() string
}

// InstanceAuth resolves X-Instance-Id to an instance and validates the
// X-Api-Key header against the instance's API key. The manager is responsible
// for applying the default API key at instance creation/restore time.
func InstanceAuth(resolver InstanceResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			instanceID := r.Header.Get("X-Instance-Id")
			if instanceID == "" {
				jsonError(w, "missing X-Instance-Id header", http.StatusBadRequest)
				return
			}

			inst, ok := resolver.GetInstance(instanceID)
			if !ok {
				jsonError(w, "instance not found", http.StatusNotFound)
				return
			}

			// Validate API key
			token := r.Header.Get("X-Api-Key")
			expectedKey := inst.GetAPIKey()

			if expectedKey != "" && token != expectedKey {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), InstanceKey, inst)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePaired rejects requests when the resolved instance does not have
// a paired WhatsApp device. Apply after InstanceAuth; skip for /session routes.
func RequirePaired(next http.Handler) http.Handler {
	type pairedChecker interface {
		IsPaired() bool
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst, ok := r.Context().Value(InstanceKey).(pairedChecker)
		if !ok || !inst.IsPaired() {
			jsonError(w, "device not paired", http.StatusConflict)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireService rejects requests when the resolved instance does not have
// an initialised Service. Apply after InstanceAuth; safe for session routes.
func RequireService(next http.Handler) http.Handler {
	type serviceChecker interface {
		HasService() bool
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst, ok := r.Context().Value(InstanceKey).(serviceChecker)
		if !ok || !inst.HasService() {
			jsonError(w, "instance not available", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}
