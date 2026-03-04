package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/instance"
)

type InstanceHandler struct {
	Handler
	mgr *instance.Manager
	cfg *config.Config
}

func NewInstanceHandler(mgr *instance.Manager, cfg *config.Config, logger *slog.Logger) *InstanceHandler {
	return &InstanceHandler{
		Handler: Handler{Logger: logger},
		mgr:     mgr,
		cfg:     cfg,
	}
}

func (h *InstanceHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Delete("/{id}", h.Delete)
	r.Put("/{id}/config", h.UpdateConfig)
	r.Put("/{id}/restart", h.Restart)
}

type CreateInstanceRequest struct {
	ID            string   `json:"id" validate:"required,instance_id"`
	WebhookURL    string   `json:"webhookUrl,omitempty" validate:"omitempty,url"`
	APIKey        string   `json:"apiKey,omitempty"`
	SigningSecret string   `json:"signingSecret,omitempty"`
	EventFilters  []string `json:"eventFilters,omitempty"`
	HistorySync   *bool    `json:"historySync,omitempty"`
}

func (h *InstanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateInstanceRequest
	if !h.Decode(w, r, &req) {
		return
	}

	cfg := config.InstanceConfig{
		APIKey:        req.APIKey,
		WebhookURL:    req.WebhookURL,
		SigningSecret: req.SigningSecret,
		EventFilters:  req.EventFilters,
		HistorySync:   req.HistorySync,
	}

	inst, err := h.mgr.CreateInstance(r.Context(), req.ID, cfg)
	if err != nil {
		h.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.JSON(w, http.StatusCreated, map[string]any{
		"id":     inst.ID,
		"config": inst.Config,
	})
}

func (h *InstanceHandler) List(w http.ResponseWriter, r *http.Request) {
	instances := h.mgr.ListInstances()
	result := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		item := map[string]any{
			"id":          inst.ID,
			"config":      inst.Config,
			"isConnected": false,
			"isLoggedIn":  false,
		}
		if inst.Service != nil {
			item["isConnected"] = inst.Service.IsConnected()
			item["isLoggedIn"] = inst.Service.IsLoggedIn()
			item["deviceId"] = inst.Service.GetDeviceJID()
		}
		result = append(result, item)
	}
	h.JSON(w, http.StatusOK, result)
}

func (h *InstanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inst, ok := h.mgr.GetInstanceDirect(id)
	if !ok {
		h.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	result := map[string]any{
		"id":          inst.ID,
		"config":      inst.Config,
		"isConnected": false,
		"isLoggedIn":  false,
	}
	if inst.Service != nil {
		result["isConnected"] = inst.Service.IsConnected()
		result["isLoggedIn"] = inst.Service.IsLoggedIn()
		result["deviceId"] = inst.Service.GetDeviceJID()
	}
	h.JSON(w, http.StatusOK, result)
}

func (h *InstanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.mgr.DeleteInstance(r.Context(), id); err != nil {
		h.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.NoContent(w)
}

func (h *InstanceHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var cfg config.InstanceConfig
	if !h.Decode(w, r, &cfg) {
		return
	}

	if err := h.mgr.UpdateInstanceConfig(r.Context(), id, cfg); err != nil {
		h.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.NoContent(w)
}

func (h *InstanceHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.mgr.RestartInstance(r.Context(), id); err != nil {
		h.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.NoContent(w)
}
