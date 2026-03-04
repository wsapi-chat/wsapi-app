package handler

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type NewsletterHandler struct {
	Handler
}

func NewNewsletterHandler(logger *slog.Logger) *NewsletterHandler {
	return &NewsletterHandler{Handler: Handler{Logger: logger}}
}

func (h *NewsletterHandler) RegisterRoutes(r chi.Router) {
	r.Route("/newsletters", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/invite/{code}", h.GetByInviteCode)
		r.Get("/{id}", h.Get)
		r.Put("/{id}/subscription", h.SetSubscription)
		r.Put("/{id}/mute", h.ToggleMute)
	})
}

type CreateNewsletterRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description,omitempty"`
	Picture     string `json:"picture,omitempty"`
}

type SetSubscriptionRequest struct {
	Subscribed bool `json:"subscribed"`
}

type ToggleMuteNewsletterRequest struct {
	Mute bool `json:"mute"`
}

func (h *NewsletterHandler) List(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	newsletters, err := inst.Service.Newsletters.GetSubscribedNewsletters(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, newsletters)
}

func (h *NewsletterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateNewsletterRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	var picture []byte
	if req.Picture != "" {
		var err error
		picture, err = base64.StdEncoding.DecodeString(req.Picture)
		if err != nil {
			h.Error(w, "invalid base64 picture", http.StatusBadRequest)
			return
		}
	}
	info, err := inst.Service.Newsletters.CreateNewsletter(r.Context(), req.Name, req.Description, picture)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, info.ID)
}

func (h *NewsletterHandler) Get(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	info, err := inst.Service.Newsletters.GetNewsletterInfo(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *NewsletterHandler) GetByInviteCode(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	code := chi.URLParam(r, "code")
	info, err := inst.Service.Newsletters.GetNewsletterInfoWithInvite(r.Context(), code)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *NewsletterHandler) SetSubscription(w http.ResponseWriter, r *http.Request) {
	var req SetSubscriptionRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id := chi.URLParam(r, "id")
	var err error
	if req.Subscribed {
		err = inst.Service.Newsletters.FollowNewsletter(r.Context(), id)
	} else {
		err = inst.Service.Newsletters.UnfollowNewsletter(r.Context(), id)
	}
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *NewsletterHandler) ToggleMute(w http.ResponseWriter, r *http.Request) {
	var req ToggleMuteNewsletterRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Newsletters.ToggleMuteNewsletter(r.Context(), chi.URLParam(r, "id"), req.Mute); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
