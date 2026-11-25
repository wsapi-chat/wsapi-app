package publisher

import (
	"context"
	"encoding/json"
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

func (p *NoopPublisher) Publish(ctx context.Context, evt event.Event) error {
	if !p.logger.Enabled(ctx, slog.LevelDebug) {
		return nil
	}
	data, _ := json.Marshal(evt.Data)
	p.logger.Debug("event discarded (no publisher configured)",
		"eventType", evt.EventType,
		"eventId", evt.EventID,
		eventDataAttr(data),
	)
	return nil
}

func (p *NoopPublisher) Close() error { return nil }
