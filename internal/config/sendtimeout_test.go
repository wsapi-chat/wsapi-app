package config_test

import (
	"testing"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// The deadline only helps if it is the binding one. Above whatsmeow's fallback
// it never fires; above the write deadline the handler's error never reaches
// the socket, which is the dropped connection WriteTimeoutDuration exists to
// avoid. It has to be the shortest of the three.
func TestSendTimeoutDefaultIsTheBindingLimit(t *testing.T) {
	var unset config.WhatsmeowConfig
	got := unset.SendTimeoutDuration()

	if got >= whatsapp.SendAckTimeout {
		t.Errorf("default SendTimeoutDuration() = %s, want less than whatsmeow's %s fallback, or it never fires",
			got, whatsapp.SendAckTimeout)
	}

	var server config.ServerConfig
	if got >= server.WriteTimeoutDuration() {
		t.Errorf("default SendTimeoutDuration() = %s, want less than the %s write deadline, or the error never reaches the caller",
			got, server.WriteTimeoutDuration())
	}
}

// Observed in real traffic: the slowest send that succeeded
// took 8.32s. A default at or under that would start failing real traffic.
func TestSendTimeoutDefaultClearsTheObservedTail(t *testing.T) {
	const slowestObservedSuccess = 8320 * time.Millisecond

	var unset config.WhatsmeowConfig
	if got := unset.SendTimeoutDuration(); got <= slowestObservedSuccess {
		t.Errorf("default SendTimeoutDuration() = %s, want more than the %s slowest successful send",
			got, slowestObservedSuccess)
	}
}

func TestSendTimeoutRespectsExplicitValue(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		note string
	}{
		{"30s", 30 * time.Second, "explicit value is honoured"},
		{"1m", time.Minute, "minutes parse"},
		{"", 20 * time.Second, "empty falls back to the default"},
		{"garbage", 20 * time.Second, "unparseable falls back to the default"},
	}

	for _, c := range cases {
		w := config.WhatsmeowConfig{SendTimeout: c.in}
		if got := w.SendTimeoutDuration(); got != c.want {
			t.Errorf("WhatsmeowConfig{SendTimeout: %q}.SendTimeoutDuration() = %s, want %s (%s)",
				c.in, got, c.want, c.note)
		}
	}
}
