package publisher

import (
	"context"
	"log/slog"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// Publisher delivers events to external consumers.
type Publisher interface {
	Publish(ctx context.Context, evt event.Event) error
	Close() error
}

// Factory creates publishers based on instance configuration.
type Factory struct {
	cfg    *config.Config
	logger *slog.Logger
	redis  *RedisPublisher // shared across instances, nil if not configured
}

// NewFactory creates a publisher factory. If Redis is configured, it initializes
// a shared Redis publisher.
func NewFactory(cfg *config.Config, logger *slog.Logger) *Factory {
	f := &Factory{cfg: cfg, logger: logger}

	if cfg.Redis != nil && cfg.Redis.URL != "" {
		rp, err := NewRedisPublisher(cfg.Redis, logger)
		if err != nil {
			logger.Warn("failed to initialize Redis publisher", "error", err)
		} else {
			f.redis = rp
		}
	}

	return f
}

// Create returns a Publisher for the given instance based on the global
// publishVia setting. The caller (manager) resolves defaults before calling.
func (f *Factory) Create(instanceID, webhookURL, signingSecret string) Publisher {
	switch f.cfg.EventsPublishVia {
	case "webhook":
		if webhookURL != "" {
			return NewWebhookPublisher(instanceID, webhookURL, signingSecret, f.logger)
		}
		f.logger.Warn("publishVia is webhook but no webhook URL configured, falling back to noop", "instanceId", instanceID)

	case "redis":
		if f.redis != nil {
			return &redisWrapper{
				rp:         f.redis,
				instanceID: instanceID,
				signature:  signingSecret,
			}
		}
		f.logger.Warn("publishVia is redis but Redis is not configured, falling back to noop", "instanceId", instanceID)
	}

	return NewNoopPublisher(f.logger)
}

// Close shuts down shared resources (e.g. Redis connection).
func (f *Factory) Close() error {
	if f.redis != nil {
		return f.redis.Close()
	}
	return nil
}
