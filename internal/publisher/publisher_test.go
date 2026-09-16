package publisher

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/event"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// With publishing disabled the instance must still get a system-events-only
// publisher rather than a plain no-op, so pairing and session state keep
// reaching any consumer.
func TestCreateWrapsSystemOnlyWhenPublishingDisabled(t *testing.T) {
	for _, via := range []string{"webhook", "redis", "none"} {
		t.Run(via, func(t *testing.T) {
			f := NewFactory(&config.Config{EventsPublishVia: via}, testLogger())
			defer f.Close() //nolint:errcheck

			pub := f.Create("ins_test", "https://example.com/hook", "secret", false)
			if _, ok := pub.(*SystemEventsOnlyPublisher); !ok {
				t.Fatalf("publishEvents=false via %q: expected *SystemEventsOnlyPublisher, got %T", via, pub)
			}
		})
	}
}

// recordingPublisher captures what actually reaches the transport.
type recordingPublisher struct{ got []string }

func (r *recordingPublisher) Publish(_ context.Context, evt event.Event) error {
	r.got = append(r.got, evt.EventType)
	return nil
}
func (r *recordingPublisher) Close() error { return nil }

// The regression this guards: suppressing an instance must not swallow
// logged_in/logged_out, which report pairing --
// independently of whether a webhook is configured.
func TestSystemOnlyPublisherForwardsSystemEventsAndDropsTheRest(t *testing.T) {
	rec := &recordingPublisher{}
	pub := NewSystemEventsOnlyPublisher(rec, testLogger(), "ins_test")

	for _, et := range []string{
		event.TypeLoggedIn, event.TypeLoggedOut, "message", "chat_presence", event.TypeInitialSyncFinished,
	} {
		if err := pub.Publish(context.Background(), event.Event{EventType: et}); err != nil {
			t.Fatalf("publish %s: %v", et, err)
		}
	}

	for _, want := range []string{event.TypeLoggedIn, event.TypeLoggedOut, event.TypeInitialSyncFinished} {
		if !slices.Contains(rec.got, want) {
			t.Errorf("system event %q was dropped; forwarded=%v", want, rec.got)
		}
	}
	for _, unwanted := range []string{"message", "chat_presence"} {
		if slices.Contains(rec.got, unwanted) {
			t.Errorf("data event %q should have been dropped; forwarded=%v", unwanted, rec.got)
		}
	}
}

// Publishing enabled with a webhook URL must still produce a webhook publisher,
// so the flag does not change behaviour for instances that have a target.
func TestCreateReturnsWebhookWhenPublishingEnabled(t *testing.T) {
	f := NewFactory(&config.Config{EventsPublishVia: "webhook"}, testLogger())
	defer f.Close() //nolint:errcheck

	pub := f.Create("ins_test", "https://example.com/hook", "secret", true)
	if _, ok := pub.(*WebhookPublisher); !ok {
		t.Fatalf("expected *WebhookPublisher, got %T", pub)
	}
}

// An instance with publishing enabled but no webhook URL keeps the existing
// behaviour: nothing to deliver to, so the no-op publisher is used.
func TestCreateReturnsNoopWhenWebhookURLMissing(t *testing.T) {
	f := NewFactory(&config.Config{EventsPublishVia: "webhook"}, testLogger())
	defer f.Close() //nolint:errcheck

	pub := f.Create("ins_test", "", "secret", true)
	if _, ok := pub.(*NoopPublisher); !ok {
		t.Fatalf("expected *NoopPublisher, got %T", pub)
	}
}
