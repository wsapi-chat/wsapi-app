package publisher

import (
	"context"
	"log/slog"

	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// SystemEventsOnlyPublisher forwards system events to the wrapped publisher and
// discards everything else.
//
// It is used for instances with no delivery target: nothing consumes their data
// events, but system events (logged_in, logged_out, login_error,
// initial_sync_finished) must still be delivered, since they report pairing and
// session state independently of whether a webhook is configured.
type SystemEventsOnlyPublisher struct {
	inner  Publisher
	logger *slog.Logger
}

func NewSystemEventsOnlyPublisher(inner Publisher, logger *slog.Logger, instanceID string) *SystemEventsOnlyPublisher {
	return &SystemEventsOnlyPublisher{
		inner:  inner,
		logger: logger.With("publisher", "system-only", "instanceId", instanceID),
	}
}

func (p *SystemEventsOnlyPublisher) Publish(ctx context.Context, evt event.Event) error {
	if !event.IsSystemEvent(evt.EventType) {
		p.logger.Debug("event discarded (publishing disabled for instance)",
			"eventType", evt.EventType,
			"eventId", evt.EventID,
		)
		return nil
	}
	return p.inner.Publish(ctx, evt)
}

func (p *SystemEventsOnlyPublisher) Close() error { return p.inner.Close() }
