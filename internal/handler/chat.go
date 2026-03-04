package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ChatHandler struct {
	Handler
}

func NewChatHandler(logger *slog.Logger) *ChatHandler {
	return &ChatHandler{Handler: Handler{Logger: logger}}
}

func (h *ChatHandler) RegisterRoutes(r chi.Router) {
	r.Route("/chats", func(r chi.Router) {
		r.Get("/", h.List)
		r.Route("/{chatId}", func(r chi.Router) {
			r.Get("/", h.GetInfo)
			r.Get("/picture", h.GetPicture)
			r.Get("/business", h.GetBusinessProfile)
			r.Put("/presence", h.SetPresence)
			r.Put("/presence/subscribe", h.SubscribePresence)
			r.Put("/ephemeral", h.SetEphemeral)
			r.Put("/mute", h.Mute)
			r.Put("/pin", h.Pin)
			r.Put("/archive", h.Archive)
			r.Put("/read", h.MarkAsRead)
			r.Post("/messages", h.RequestMessages)
			r.Post("/clear", h.Clear)
			r.Delete("/", h.Delete)
		})
	})
}

func (h *ChatHandler) List(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	chats, err := inst.Service.Chats.ListChats(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, chats)
}

func (h *ChatHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	chatID := chi.URLParam(r, "chatId")
	info, err := inst.Service.Chats.GetChatInfo(r.Context(), chatID)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, info)
}

type SetPresenceRequest struct {
	State string `json:"state" validate:"required,oneof=typing paused recording"`
}

type SetEphemeralRequest struct {
	Expiration string `json:"expiration" validate:"required,oneof=off 24h 7d 90d"`
}

type MuteRequest struct {
	Duration string `json:"duration" validate:"required,oneof=8h 1w always off"`
}

type PinRequest struct {
	Pinned bool `json:"pinned"`
}

type ArchiveRequest struct {
	Archived bool `json:"archived"`
}

type ChatMarkAsReadRequest struct {
	Read bool `json:"read"`
}

type RequestMessagesRequest struct {
	LastMessageID       string `json:"lastMessageId" validate:"required"`
	LastMessageSenderID string `json:"lastMessageSenderId" validate:"required"`
	Count               int    `json:"count,omitempty" validate:"omitempty,min=1,max=500"`
}

func (h *ChatHandler) GetPicture(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	picture, err := inst.Service.Chats.GetChatPicture(r.Context(), chi.URLParam(r, "chatId"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, picture)
}

func (h *ChatHandler) GetBusinessProfile(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	profile, err := inst.Service.Chats.GetBusinessProfile(r.Context(), chi.URLParam(r, "chatId"))
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, profile)
}

func (h *ChatHandler) SetPresence(w http.ResponseWriter, r *http.Request) {
	var req SetPresenceRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.SendChatPresence(r.Context(), chi.URLParam(r, "chatId"), req.State); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) SubscribePresence(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Chats.SubscribeChatPresence(r.Context(), chi.URLParam(r, "chatId")); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) SetEphemeral(w http.ResponseWriter, r *http.Request) {
	var req SetEphemeralRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.SetEphemeralExpiration(r.Context(), chi.URLParam(r, "chatId"), req.Expiration); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) Mute(w http.ResponseWriter, r *http.Request) {
	var req MuteRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.MuteChat(r.Context(), chi.URLParam(r, "chatId"), req.Duration); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) Pin(w http.ResponseWriter, r *http.Request) {
	var req PinRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.PinChat(r.Context(), chi.URLParam(r, "chatId"), req.Pinned); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) Archive(w http.ResponseWriter, r *http.Request) {
	var req ArchiveRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.ArchiveChat(r.Context(), chi.URLParam(r, "chatId"), req.Archived); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	var req ChatMarkAsReadRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Chats.MarkChatAsRead(r.Context(), chi.URLParam(r, "chatId"), req.Read); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) RequestMessages(w http.ResponseWriter, r *http.Request) {
	var req RequestMessagesRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	count := req.Count
	if count == 0 {
		count = 50
	}
	if err := inst.Service.Chats.RequestMessages(r.Context(), chi.URLParam(r, "chatId"), req.LastMessageID, req.LastMessageSenderID, count); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (h *ChatHandler) Clear(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Chats.ClearChat(r.Context(), chi.URLParam(r, "chatId")); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ChatHandler) Delete(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Chats.DeleteChat(r.Context(), chi.URLParam(r, "chatId")); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
