package publisher

import (
	"context"
	"log/slog"

	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// NoopPublisher logs events but does not deliver them anywhere.
type NoopPublisher struct {
	logger *slog.Logger
}

func NewNoopPublisher(logger *slog.Logger) *NoopPublisher {
	return &NoopPublisher{logger: logger.With("publisher", "noop")}
}

func (p *NoopPublisher) Publish(_ context.Context, evt event.Event) error {
	p.logger.Debug("event discarded (no publisher configured)",
		"eventType", evt.EventType,
		"eventId", evt.EventID,
		"eventData", evt.Data,
	)
	return nil
}

func (p *NoopPublisher) Close() error { return nil }
