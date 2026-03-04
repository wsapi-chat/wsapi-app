package handler

import (
	"log/slog"
	"net/http"

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
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
