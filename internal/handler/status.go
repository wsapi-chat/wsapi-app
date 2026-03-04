package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *StatusHandler) Delete(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	if err := inst.Service.Status.DeleteStatus(r.Context(), messageID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

type StatusHandler struct {
	Handler
}

func NewStatusHandler(logger *slog.Logger) *StatusHandler {
	return &StatusHandler{Handler: Handler{Logger: logger}}
}

func (h *StatusHandler) RegisterRoutes(r chi.Router) {
	r.Route("/status", func(r chi.Router) {
		r.Get("/privacy", h.GetPrivacy)
		r.Post("/text", h.PostText)
		r.Post("/image", h.PostImage)
		r.Post("/video", h.PostVideo)
		r.Post("/{messageId}/delete", h.Delete)
	})
}

func (h *StatusHandler) GetPrivacy(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	privacy, err := inst.Service.Status.GetStatusPrivacy(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, privacy)
}

type PostTextStatusRequest struct {
	Text string `json:"text" validate:"required"`
}

type PostMediaStatusRequest struct {
	MediaData
	MimeType string `json:"mimeType,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

func (h *StatusHandler) PostText(w http.ResponseWriter, r *http.Request) {
	var req PostTextStatusRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Status.PostTextStatus(r.Context(), req.Text)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *StatusHandler) PostImage(w http.ResponseWriter, r *http.Request) {
	var req PostMediaStatusRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Status.PostImageStatus(r.Context(), data, req.MimeType, req.Caption)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *StatusHandler) PostVideo(w http.ResponseWriter, r *http.Request) {
	var req PostMediaStatusRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Status.PostVideoStatus(r.Context(), data, req.MimeType, req.Caption)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}
