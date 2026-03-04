package handler

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/validate"
)

type UserHandler struct {
	Handler
}

func NewUserHandler(logger *slog.Logger) *UserHandler {
	return &UserHandler{Handler: Handler{Logger: logger}}
}

func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Route("/me", func(r chi.Router) {
			r.Get("/profile", h.GetMyProfile)
			r.Put("/profile", h.UpdateMyProfile)
			r.Put("/presence", h.SetPresence)
			r.Get("/privacy", h.GetPrivacy)
			r.Put("/privacy", h.SetPrivacy)
		})
		r.Post("/check", h.BulkCheck)
		r.Get("/{phone}/check", h.Check)
		r.Get("/{phone}/profile", h.Profile)
	})
}

type UpdateProfileRequest struct {
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Picture string `json:"picture,omitempty"`
}

type SetMyPresenceRequest struct {
	Presence string `json:"presence" validate:"required,oneof=available unavailable"`
}

type BulkCheckRequest struct {
	Phones []string `json:"phones" validate:"required,min=1"`
}

type SetPrivacyRequest struct {
	Setting string `json:"setting" validate:"required,oneof=groupadd last status profile readreceipts online calladd"`
	Value   string `json:"value" validate:"required,oneof=all contacts contact_blacklist match_last_seen known none"`
}

func (h *UserHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	info, err := inst.Service.Account.GetMyInfo(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *UserHandler) UpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)

	if req.Name != "" {
		if err := inst.Service.Account.SetName(r.Context(), req.Name); err != nil {
			h.ServiceError(w, err)
			return
		}
	}

	if req.Status != "" {
		if err := inst.Service.Account.SetStatus(r.Context(), req.Status); err != nil {
			h.ServiceError(w, err)
			return
		}
	}

	if req.Picture != "" {
		data, err := base64.StdEncoding.DecodeString(req.Picture)
		if err != nil {
			h.Error(w, "invalid base64 picture", http.StatusBadRequest)
			return
		}
		if _, err := inst.Service.Account.SetProfilePicture(r.Context(), data); err != nil {
			h.ServiceError(w, err)
			return
		}
	}

	h.NoContent(w)
}

func (h *UserHandler) SetPresence(w http.ResponseWriter, r *http.Request) {
	var req SetMyPresenceRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)

	if err := inst.Service.Account.SendPresence(r.Context(), req.Presence); err != nil {
		h.ServiceError(w, err)
		return
	}

	h.NoContent(w)
}

func (h *UserHandler) Check(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	phone := chi.URLParam(r, "phone")
	if !validate.Phone(phone) {
		h.Error(w, "invalid phone number format", http.StatusBadRequest)
		return
	}
	onWhatsApp, err := inst.Service.Users.CheckOnWhatsApp(r.Context(), phone)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]bool{"isInWhatsApp": onWhatsApp})
}

func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	phone := chi.URLParam(r, "phone")
	if !validate.Phone(phone) {
		h.Error(w, "invalid phone number format", http.StatusBadRequest)
		return
	}
	info, err := inst.Service.Users.GetUserInfo(r.Context(), phone)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

func (h *UserHandler) BulkCheck(w http.ResponseWriter, r *http.Request) {
	var req BulkCheckRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	results, err := inst.Service.Users.BulkCheckOnWhatsApp(r.Context(), req.Phones)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, results)
}

func (h *UserHandler) GetPrivacy(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	settings, err := inst.Service.Account.GetPrivacySettings(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, settings)
}

func (h *UserHandler) SetPrivacy(w http.ResponseWriter, r *http.Request) {
	var req SetPrivacyRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	settings, err := inst.Service.Account.SetPrivacySetting(r.Context(), req.Setting, req.Value)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, settings)
}
