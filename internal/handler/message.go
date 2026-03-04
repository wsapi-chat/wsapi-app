package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

type MessageHandler struct {
	Handler
}

func NewMessageHandler(logger *slog.Logger) *MessageHandler {
	return &MessageHandler{Handler: Handler{Logger: logger}}
}

func (h *MessageHandler) RegisterRoutes(r chi.Router) {
	r.Route("/messages", func(r chi.Router) {
		r.Post("/text", h.SendText)
		r.Post("/image", h.SendImage)
		r.Post("/video", h.SendVideo)
		r.Post("/audio", h.SendAudio)
		r.Post("/voice", h.SendVoice)
		r.Post("/document", h.SendDocument)
		r.Post("/sticker", h.SendSticker)
		r.Post("/contact", h.SendContact)
		r.Post("/location", h.SendLocation)
		r.Post("/link", h.SendLink)

		r.Post("/{messageId}/reaction", h.SendReaction)
		r.Post("/{messageId}/edit", h.EditMessage)
		r.Post("/{messageId}/read", h.MarkAsRead)
		r.Post("/{messageId}/star", h.StarMessage)
		r.Post("/{messageId}/pin", h.PinMessage)
		r.Post("/{messageId}/delete", h.DeleteMessage)
		r.Post("/{messageId}/delete-for-me", h.DeleteMessageForMe)
	})
}

// Shared types

// MediaData provides common fields and logic for resolving media from base64 data or URL.
type MediaData struct {
	Data string `json:"data,omitempty"`
	URL  string `json:"url,omitempty" validate:"omitempty,url"`
}

func (m *MediaData) resolveData(ctx context.Context) ([]byte, error) {
	if m.Data != "" && m.URL != "" {
		return nil, fmt.Errorf("provide either data or url, not both")
	}
	if m.Data != "" {
		return base64.StdEncoding.DecodeString(m.Data)
	}
	if m.URL != "" {
		return whatsapp.DownloadMediaFromURL(ctx, m.URL)
	}
	return nil, fmt.Errorf("either data or url is required")
}

// CommonSendOptions provides common fields shared across message send requests.
type CommonSendOptions struct {
	Mentions            []string `json:"mentions,omitempty"`
	ReplyTo             string   `json:"replyTo,omitempty"`
	ReplyToSenderID     string   `json:"replyToSenderId,omitempty"`
	IsForwarded         bool     `json:"isForwarded,omitempty"`
	EphemeralExpiration string   `json:"ephemeralExpiration,omitempty" validate:"omitempty,oneof=off 24h 7d 90d"`
}

func (o *CommonSendOptions) toSendOptions() whatsapp.SendOptions {
	return whatsapp.SendOptions{
		Mentions:            o.Mentions,
		ReplyTo:             o.ReplyTo,
		ReplyToSenderID:     o.ReplyToSenderID,
		IsForwarded:         o.IsForwarded,
		EphemeralExpiration: o.EphemeralExpiration,
	}
}

// Request types

type SendTextRequest struct {
	CommonSendOptions
	To   string `json:"to" validate:"required"`
	Text string `json:"text" validate:"required"`
}

type SendMediaRequest struct {
	CommonSendOptions
	MediaData
	To       string `json:"to" validate:"required"`
	MimeType string `json:"mimeType,omitempty"`
	Caption  string `json:"caption,omitempty"`
	ViewOnce bool   `json:"viewOnce,omitempty"`
}

func (r *SendMediaRequest) toMediaSendOptions() whatsapp.MediaSendOptions {
	return whatsapp.MediaSendOptions{
		SendOptions: r.toSendOptions(),
		Caption:     r.Caption,
		ViewOnce:    r.ViewOnce,
	}
}

type SendDocumentRequest struct {
	SendMediaRequest
	Filename string `json:"filename" validate:"required"`
}

type SendStickerRequest struct {
	CommonSendOptions
	MediaData
	To         string `json:"to" validate:"required"`
	IsAnimated bool   `json:"isAnimated,omitempty"`
}

type SendContactRequest struct {
	CommonSendOptions
	To          string `json:"to" validate:"required"`
	DisplayName string `json:"displayName,omitempty" validate:"required_without=VCard"`
	VCard       string `json:"vcard,omitempty" validate:"required_without=DisplayName"`
}

type SendLocationRequest struct {
	To                  string  `json:"to" validate:"required"`
	Latitude            float64 `json:"latitude" validate:"min=-90,max=90"`
	Longitude           float64 `json:"longitude" validate:"min=-180,max=180"`
	Name                string  `json:"name,omitempty"`
	Address             string  `json:"address,omitempty"`
	URL                 string  `json:"url,omitempty"`
	EphemeralExpiration string  `json:"ephemeralExpiration,omitempty" validate:"omitempty,oneof=off 24h 7d 90d"`
}

type SendLinkRequest struct {
	CommonSendOptions
	To            string `json:"to" validate:"required"`
	Text          string `json:"text" validate:"required"`
	URL           string `json:"url" validate:"required,url"`
	Title         string `json:"title,omitempty"`
	Description   string `json:"description,omitempty"`
	JPEGThumbnail string `json:"jpegThumbnail,omitempty"`
}

type SendReactionRequest struct {
	To       string `json:"to" validate:"required"`
	SenderID string `json:"senderId,omitempty"`
	Reaction string `json:"reaction"`
}

type EditMessageRequest struct {
	To                  string   `json:"to" validate:"required"`
	Text                string   `json:"text" validate:"required"`
	Mentions            []string `json:"mentions,omitempty"`
	EphemeralExpiration string   `json:"ephemeralExpiration,omitempty" validate:"omitempty,oneof=off 24h 7d 90d"`
}

func (r *EditMessageRequest) toSendOptions() whatsapp.SendOptions {
	return whatsapp.SendOptions{
		Mentions:            r.Mentions,
		EphemeralExpiration: r.EphemeralExpiration,
	}
}

type MarkAsReadRequest struct {
	ChatID      string `json:"chatId" validate:"required"`
	SenderID    string `json:"senderId" validate:"required"`
	ReceiptType string `json:"receiptType" validate:"required,oneof=delivered sender read played"`
}

type StarMessageRequest struct {
	ChatID   string `json:"chatId" validate:"required"`
	SenderID string `json:"senderId" validate:"required"`
	Starred  bool   `json:"starred"`
}

type DeleteMessageRequest struct {
	ChatID   string `json:"chatId" validate:"required"`
	SenderID string `json:"senderId" validate:"required"`
}

type DeleteMessageForMeRequest struct {
	ChatID    string    `json:"chatId" validate:"required"`
	SenderID  string    `json:"senderId,omitempty"`
	IsFromMe  bool      `json:"isFromMe,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

type PinMessageRequest struct {
	ChatID        string `json:"chatId" validate:"required"`
	SenderID      string `json:"senderId" validate:"required"`
	Pinned        bool   `json:"pinned"`
	PinExpiration string `json:"pinExpiration,omitempty"`
}

// Handlers

func (h *MessageHandler) SendText(w http.ResponseWriter, r *http.Request) {
	var req SendTextRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Messages.SendText(r.Context(), req.To, req.Text, req.toSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendImage(w http.ResponseWriter, r *http.Request) {
	var req SendMediaRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendImage(r.Context(), req.To, data, req.MimeType, req.toMediaSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendVideo(w http.ResponseWriter, r *http.Request) {
	var req SendMediaRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendVideo(r.Context(), req.To, data, req.MimeType, req.toMediaSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendAudio(w http.ResponseWriter, r *http.Request) {
	var req SendMediaRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendAudio(r.Context(), req.To, data, req.MimeType, req.toMediaSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendVoice(w http.ResponseWriter, r *http.Request) {
	var req SendMediaRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendVoice(r.Context(), req.To, data, req.toMediaSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendDocument(w http.ResponseWriter, r *http.Request) {
	var req SendDocumentRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendDocument(r.Context(), req.To, data, req.Filename, req.toMediaSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendSticker(w http.ResponseWriter, r *http.Request) {
	var req SendStickerRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	data, err := req.resolveData(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	id, err := inst.Service.Messages.SendSticker(r.Context(), req.To, data, req.IsAnimated, req.toSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendContact(w http.ResponseWriter, r *http.Request) {
	var req SendContactRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Messages.SendContact(r.Context(), req.To, req.DisplayName, req.VCard, req.toSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendLocation(w http.ResponseWriter, r *http.Request) {
	var req SendLocationRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	id, err := inst.Service.Messages.SendLocation(r.Context(), req.To, req.Latitude, req.Longitude, req.Name, req.Address, req.URL, req.EphemeralExpiration)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendLink(w http.ResponseWriter, r *http.Request) {
	var req SendLinkRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	var thumbnail []byte
	if req.JPEGThumbnail != "" {
		var err error
		thumbnail, err = base64.StdEncoding.DecodeString(req.JPEGThumbnail)
		if err != nil {
			h.Error(w, "invalid base64 thumbnail", http.StatusBadRequest)
			return
		}
	}
	id, err := inst.Service.Messages.SendLink(r.Context(), req.To, req.Text, req.URL, req.Title, req.Description, thumbnail, req.toSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.Created(w, id)
}

func (h *MessageHandler) SendReaction(w http.ResponseWriter, r *http.Request) {
	var req SendReactionRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	id, err := inst.Service.Messages.SendReaction(r.Context(), req.To, req.SenderID, messageID, req.Reaction)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"id": id})
}

func (h *MessageHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	var req EditMessageRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	id, err := inst.Service.Messages.EditMessage(r.Context(), req.To, messageID, req.Text, req.toSendOptions())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"id": id})
}

func (h *MessageHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	var req MarkAsReadRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	if err := inst.Service.Messages.MarkAsRead(r.Context(), req.ChatID, req.SenderID, messageID, req.ReceiptType); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *MessageHandler) StarMessage(w http.ResponseWriter, r *http.Request) {
	var req StarMessageRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	if err := inst.Service.Messages.StarMessage(r.Context(), req.ChatID, req.SenderID, messageID, req.Starred); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *MessageHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	var req PinMessageRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	if err := inst.Service.Messages.PinMessage(r.Context(), req.ChatID, req.SenderID, messageID, req.Pinned, req.PinExpiration); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	var req DeleteMessageRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	if err := inst.Service.Messages.DeleteMessage(r.Context(), req.ChatID, req.SenderID, messageID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *MessageHandler) DeleteMessageForMe(w http.ResponseWriter, r *http.Request) {
	var req DeleteMessageForMeRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	messageID := chi.URLParam(r, "messageId")
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	if err := inst.Service.Messages.DeleteMessageForMe(r.Context(), req.ChatID, req.SenderID, req.IsFromMe, messageID, ts); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}
