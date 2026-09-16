package config_test

import (
	"testing"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// A stalled send holds the handler for whatsmeow's full send-ack timeout. If the
// write deadline expires first, the handler's error response never reaches the
// socket and the caller gets a dropped connection instead of a status. The
// default must therefore stay above SendAckTimeout.
func TestWriteTimeoutDefaultOutlastsSendAck(t *testing.T) {
	var unset config.ServerConfig
	got := unset.WriteTimeoutDuration()

	if got <= whatsapp.SendAckTimeout {
		t.Errorf("default WriteTimeoutDuration() = %s, want more than the %s send-ack timeout",
			got, whatsapp.SendAckTimeout)
	}
}

func TestWriteTimeoutRespectsExplicitValue(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		note string
	}{
		{"120s", 120 * time.Second, "explicit value is honoured"},
		{"2m", 2 * time.Minute, "minutes parse"},
		{"", 90 * time.Second, "empty falls back to the default"},
		{"garbage", 90 * time.Second, "unparseable falls back to the default"},
	}

	for _, c := range cases {
		s := config.ServerConfig{WriteTimeout: c.in}
		if got := s.WriteTimeoutDuration(); got != c.want {
			t.Errorf("ServerConfig{WriteTimeout: %q}.WriteTimeoutDuration() = %s, want %s (%s)",
				c.in, got, c.want, c.note)
		}
	}
}
