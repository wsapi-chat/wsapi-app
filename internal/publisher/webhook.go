package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/event"
	"github.com/wsapi-chat/wsapi-app/internal/httputil"
)

// WebhookPublisher delivers events via HTTP POST.
type WebhookPublisher struct {
	instanceID    string
	url           string
	signingSecret string
	client        *http.Client
	logger        *slog.Logger
}

// NewWebhookPublisher creates a webhook publisher for a single instance.
func NewWebhookPublisher(instanceID, url, signingSecret string, logger *slog.Logger) *WebhookPublisher {
	return &WebhookPublisher{
		instanceID:    instanceID,
		url:           url,
		signingSecret: signingSecret,
		client:        httputil.NewClient(10 * time.Second),
		logger:        logger.With("publisher", "webhook", "instanceId", instanceID),
	}
}

func (p *WebhookPublisher) Publish(ctx context.Context, evt event.Event) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	p.logger.Debug("publishing event",
		"eventType", evt.EventType,
		"eventId", evt.EventID,
		"signed", p.signingSecret != "",
		"eventData", json.RawMessage(body),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Instance-Id", p.instanceID)

	if p.signingSecret != "" {
		req.Header.Set("X-Webhook-Signature", Sign(body, p.signingSecret))
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
}

func (p *WebhookPublisher) Close() error { return nil }
