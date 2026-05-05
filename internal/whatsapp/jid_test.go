package whatsapp

import (
	"testing"

	waTypes "go.mau.fi/whatsmeow/types"
)

func TestFormatRecipient(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantUser   string
		wantServer string
	}{
		{"plain digits", "250725258789", "250725258789", waTypes.DefaultUserServer},
		{"leading plus", "+250725258789", "250725258789", waTypes.DefaultUserServer},
		{"full user JID", "250725258789@s.whatsapp.net", "250725258789", waTypes.DefaultUserServer},
		{"full user JID with leading plus", "+250725258789@s.whatsapp.net", "250725258789", waTypes.DefaultUserServer},
		{"group JID", "120363023456789012@g.us", "120363023456789012", waTypes.GroupServer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRecipient(tt.in)
			if got.User != tt.wantUser || got.Server != tt.wantServer {
				t.Errorf("FormatRecipient(%q) = %s@%s, want %s@%s",
					tt.in, got.User, got.Server, tt.wantUser, tt.wantServer)
			}
		})
	}
}
