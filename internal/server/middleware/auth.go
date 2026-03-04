package middleware

import (
	"net/http"
)

// AdminAuth validates the X-Api-Key header against the admin API key.
func AdminAuth(adminAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminAPIKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-Api-Key")
			if token == "" || token != adminAPIKey {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
