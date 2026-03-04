package handler

import (
	"context"
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
		r.Post("/logout", h.Logout)
		r.Post("/flush-history", h.FlushHistory)
	})
}

func (h *SessionHandler) QR(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	png, err := inst.Service.Session.GenerateQRImage()
	if err != nil {
		h.ServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.WriteHeader(http.StatusOK)
	w.Write(png)
}

func (h *SessionHandler) QRText(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)

	code, err := inst.Service.Session.GenerateQRCode()
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
	})
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
