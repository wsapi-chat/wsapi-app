package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"go.mau.fi/whatsmeow"

	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

func TestServiceStatus(t *testing.T) {
	h := &Handler{}

	cases := []struct {
		err  error
		want int
		note string
	}{
		{whatsapp.ErrNotFound, http.StatusNotFound, "not found"},
		{whatsapp.ErrUpstream, http.StatusBadGateway, "upstream rejected"},
		{whatsapp.ErrTooLarge, http.StatusRequestEntityTooLarge, "media over the size cap"},
		{whatsapp.ErrTimeout, http.StatusGatewayTimeout, "service-side wait exceeded"},
		{context.DeadlineExceeded, http.StatusGatewayTimeout, "context deadline"},
		{context.Canceled, statusClientClosedRequest, "caller went away"},
		{errors.New("some other failure"), http.StatusBadRequest, "unclassified falls back to 400"},

		// The case that motivated this. whatsmeow.ErrMessageTimedOut is a bare
		// errors.New: it satisfies none of the timeout sentinels above, so it
		// used to reach the default branch and get reported as a 400 — telling
		// the caller their request was malformed when WhatsApp had simply not
		// acknowledged the message.
		{whatsmeow.ErrMessageTimedOut, http.StatusGatewayTimeout, "no ack from WhatsApp"},

		// A send to a recipient with no established session fetches prekeys
		// first. That stalls on the same 75s budget and was equally invisible.
		{whatsmeow.ErrIQTimedOut, http.StatusGatewayTimeout, "info query stalled"},

		// Same error once a caller has added context around it.
		{
			fmt.Errorf("sending to %s: %w", "15550001111@s.whatsapp.net", whatsmeow.ErrMessageTimedOut),
			http.StatusGatewayTimeout,
			"wrapped send timeout",
		},
	}

	for _, c := range cases {
		if got := h.serviceStatus(c.err); got != c.want {
			t.Errorf("serviceStatus(%v) = %d, want %d (%s)", c.err, got, c.want, c.note)
		}
	}
}

// A send timeout must not be reported as a client error: 4xx tells a
// well-behaved caller not to retry, and this failure is transient.
func TestSendTimeoutIsRetryable(t *testing.T) {
	h := &Handler{}
	status := h.serviceStatus(whatsmeow.ErrMessageTimedOut)
	if status >= 400 && status < 500 {
		t.Errorf("serviceStatus(ErrMessageTimedOut) = %d, want a 5xx so callers retry", status)
	}
}

func TestIsUpstreamTimeout(t *testing.T) {
	if !whatsapp.IsUpstreamTimeout(whatsmeow.ErrMessageTimedOut) {
		t.Error("IsUpstreamTimeout(ErrMessageTimedOut) = false, want true")
	}
	if !whatsapp.IsUpstreamTimeout(fmt.Errorf("wrapped: %w", whatsmeow.ErrIQTimedOut)) {
		t.Error("IsUpstreamTimeout did not see through a wrapped error")
	}
	if whatsapp.IsUpstreamTimeout(context.DeadlineExceeded) {
		t.Error("IsUpstreamTimeout(context.DeadlineExceeded) = true, want false")
	}
	if whatsapp.IsUpstreamTimeout(nil) {
		t.Error("IsUpstreamTimeout(nil) = true, want false")
	}
}
