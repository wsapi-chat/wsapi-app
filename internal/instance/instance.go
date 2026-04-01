package instance

import (
	"context"
	"log/slog"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/event"
	"github.com/wsapi-chat/wsapi-app/internal/publisher"
	"github.com/wsapi-chat/wsapi-app/internal/server/middleware"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// Instance represents a single managed WhatsApp instance.
type Instance struct {
	ID        string
	Config    config.InstanceConfig
	Service   *whatsapp.Service
	Publisher publisher.Publisher
	Dedup     *event.Dedup
	Logger    *slog.Logger
}

// GetAPIKey returns the instance-level API key, satisfying the
// middleware.Instance interface.
func (i *Instance) GetAPIKey() string {
	return i.Config.APIKey
}

// HasService reports whether the instance has an initialised Service.
func (i *Instance) HasService() bool {
	return i.Service != nil
}

// IsPaired reports whether the instance has a paired WhatsApp device.
func (i *Instance) IsPaired() bool {
	return i.Service != nil && i.Service.IsLoggedIn()
}

// FromContext extracts the *Instance stored in the request context by the
// InstanceAuth middleware. Returns nil if not present.
func FromContext(ctx context.Context) *Instance {
	v, _ := ctx.Value(middleware.InstanceKey).(*Instance)
	return v
}
