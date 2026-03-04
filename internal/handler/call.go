package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type CallHandler struct {
	Handler
}

func NewCallHandler(logger *slog.Logger) *CallHandler {
	return &CallHandler{Handler: Handler{Logger: logger}}
}

func (h *CallHandler) RegisterRoutes(r chi.Router) {
	r.Route("/calls", func(r chi.Router) {
		r.Post("/{callId}/reject", h.Reject)
	})
}

type RejectCallRequest struct {
	CallerID string `json:"callerId" validate:"required"`
}

func (h *CallHandler) Reject(w http.ResponseWriter, r *http.Request) {
	var req RejectCallRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	callID := chi.URLParam(r, "callId")
	if err := inst.Service.Calls.RejectCall(r.Context(), req.CallerID, callID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
