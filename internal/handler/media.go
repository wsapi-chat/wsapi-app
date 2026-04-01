package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"unicode"

	"github.com/go-chi/chi/v5"
)

type MediaHandler struct {
	Handler
}

func NewMediaHandler(logger *slog.Logger) *MediaHandler {
	return &MediaHandler{Handler: Handler{Logger: logger}}
}

func (h *MediaHandler) RegisterRoutes(r chi.Router) {
	r.Route("/media", func(r chi.Router) {
		r.Get("/download", h.Download)
	})
}

func (h *MediaHandler) Download(w http.ResponseWriter, r *http.Request) {
	mediaID := r.URL.Query().Get("id")
	if mediaID == "" {
		h.Error(w, "missing required query parameter: id", http.StatusBadRequest)
		return
	}
	inst := h.Instance(r)
	data, filename, mimeType, err := inst.Service.Media.DownloadByID(r.Context(), mediaID)
	if err != nil {
		h.ServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// contentDisposition builds a Content-Disposition header value.
// For ASCII-only filenames it uses a simple filename="…" parameter.
// For non-ASCII filenames (e.g. Cyrillic) it adds the RFC 5987
// filename*=UTF-8'lang'value parameter so browsers and proxies handle them correctly.
func contentDisposition(filename string) string {
	if isASCII(filename) {
		return fmt.Sprintf("attachment; filename=%q", filename)
	}
	// ASCII fallback: use just the extension so the header stays valid.
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}
	fallback := "download" + ext
	encoded := url.PathEscape(filename)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fallback, encoded)
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
