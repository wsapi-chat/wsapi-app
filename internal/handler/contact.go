package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ContactHandler struct {
	Handler
}

func NewContactHandler(logger *slog.Logger) *ContactHandler {
	return &ContactHandler{Handler: Handler{Logger: logger}}
}

func (h *ContactHandler) RegisterRoutes(r chi.Router) {
	r.Route("/contacts", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Post("/sync", h.Sync)
		r.Get("/blocklist", h.GetBlocklist)
		r.Get("/{id}", h.Get)
		r.Put("/{id}/block", h.Block)
		r.Put("/{id}/unblock", h.Unblock)
	})
}

type CreateContactRequest struct {
	ID        string `json:"id" validate:"required"`
	FullName  string `json:"fullName" validate:"required"`
	FirstName string `json:"firstName"`
}

func (h *ContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateContactRequest
	if !h.Decode(w, r, &req) {
		return
	}
	inst := h.Instance(r)
	if err := inst.Service.Contacts.CreateOrUpdateContact(r.Context(), req.ID, req.FullName, req.FirstName); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ContactHandler) Sync(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	if err := inst.Service.Contacts.SyncContacts(r.Context()); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ContactHandler) List(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	contacts, err := inst.Service.Contacts.GetAllContacts(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, contacts)
}

func (h *ContactHandler) Get(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	contactID := chi.URLParam(r, "id")
	contact, err := inst.Service.Contacts.GetContact(r.Context(), contactID)
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, contact)
}

func (h *ContactHandler) Block(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	contactID := chi.URLParam(r, "id")
	if err := inst.Service.Contacts.BlockContact(r.Context(), contactID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ContactHandler) Unblock(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	contactID := chi.URLParam(r, "id")
	if err := inst.Service.Contacts.UnblockContact(r.Context(), contactID); err != nil {
		h.ServiceError(w, err)
		return
	}
	h.NoContent(w)
}

func (h *ContactHandler) GetBlocklist(w http.ResponseWriter, r *http.Request) {
	inst := h.Instance(r)
	blocklist, err := inst.Service.Contacts.GetBlocklist(r.Context())
	if err != nil {
		h.ServiceError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, blocklist)
}
