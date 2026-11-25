package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/validate"
)

// LogoutNotifier is called after a successful API logout to persist
// the device state change. Satisfied by *instance.Manager.
type LogoutNotifier interface {
	HandleLogout(ctx context.Context, id string)
}

type SessionHandler struct {
	Handler
	logoutNotifier LogoutNotifier
}

func NewSessionHandler(logger *slog.Logger, ln LogoutNotifier) *SessionHandler {
	return &SessionHandler{
		Handler:        Handler{Logger: logger},
		logoutNotifier: ln,
	}
}

func (h *SessionHandler) RegisterRoutes(r chi.Router) {
	r.Route("/session", func(r chi.Router) {
		r.Get("/qr", h.QR)
		r.Get("/qr/text", h.QRText)
		r.Get("/pair-code/{phone}", h.PairCode)
		r.Get("/status", h.Status)
		r.Get("/passkey", h.PasskeyState)
		r.Post("/passkey", h.SubmitPasskey)
		r.Post("/passkey/request", h.RequestPasskey)
		r.Post("/passkey/confirm", h.ConfirmPasskey)
		r.Post("/logout", h.Logout)
		r.Post("/flush-history", h.FlushHistory)
	})
}

func (h *SessionHandler) QR(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	png, err := inst.Service.Session.GenerateQRImage(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *SessionHandler) QRText(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	code, err := inst.Service.Session.GenerateQRCode(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"code": code})
}

func (h *SessionHandler) PairCode(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	phone := chi.URLParam(r, "phone")
	if !validate.Phone(phone) {
		h.Error(w, "invalid phone number format", http.StatusBadRequest)
		return
	}
	code, err := inst.Service.Session.GeneratePairCode(r.Context(), phone)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"code": code})
}

func (h *SessionHandler) Status(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	h.JSON(w, http.StatusOK, map[string]any{
		"isConnected": inst.Service.IsConnected(),
		"isLoggedIn":  inst.Service.IsLoggedIn(),
		"deviceId":    inst.Service.GetDeviceJID(),
		// Lets pollers notice a pending passkey step without a second call.
		"passkeyStatus": inst.Service.Session.PasskeyState().Status,
	})
}

// maxPasskeyResponseBytes caps the assertion body. A real one is ~1-2 KB.
const maxPasskeyResponseBytes = 64 << 10

// PasskeyState returns the pending passkey pairing step, if any. While the
// status is "required" the response carries the WebAuthn challenge to sign on
// https://web.whatsapp.com.
func (h *SessionHandler) PasskeyState(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	h.JSON(w, http.StatusOK, inst.Service.Session.PasskeyState())
}

// SubmitPasskey forwards the browser's WebAuthn assertion (the JSON from
// PublicKeyCredential.toJSON()) to WhatsApp.
func (h *SessionHandler) SubmitPasskey(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	body, err := io.ReadAll(io.LimitReader(r.Body, maxPasskeyResponseBytes+1))
	if err != nil {
		h.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(body) > maxPasskeyResponseBytes {
		h.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	state, err := inst.Service.Session.SubmitPasskeyResponse(r.Context(), body)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, state)
}

// RequestPasskey asks WhatsApp for a passkey challenge on the live pairing
// connection, for when its notification doesn't arrive on its own.
func (h *SessionHandler) RequestPasskey(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	state, err := inst.Service.Session.RequestPasskey(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, state)
}

// ConfirmPasskey finishes a passkey pairing after the user checked that the
// confirmation code matches the one on their phone.
func (h *SessionHandler) ConfirmPasskey(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	if err := inst.Service.Session.ConfirmPasskey(r.Context()); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	if err := inst.Service.Session.Logout(r.Context()); err != nil {
		h.ServiceError(w, err)
		return
	}

	// client.Logout() does not emit a LoggedOut event, so we must
	// explicitly persist the device state change.
	h.logoutNotifier.HandleLogout(r.Context(), inst.ID)

	h.NoContent(w)
}

func (h *SessionHandler) FlushHistory(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	if !inst.IsPaired() {
		h.Error(w, "device not paired", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	go func() {
		defer cancel()
		inst.Service.HistorySync.FlushHistory(ctx, inst.Publisher, inst.ID)
	}()

	h.JSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}
