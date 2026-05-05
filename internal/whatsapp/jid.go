package whatsapp

import (
	"fmt"
	"strings"

	waTypes "go.mau.fi/whatsmeow/types"
)

// parseJID parses a string JID into a whatsmeow JID.
// The input must be a full JID in user@server format (e.g. "120363...@g.us").
// Plain phone numbers or strings without '@' are rejected.
func parseJID(s string) (waTypes.JID, error) {
	if !strings.Contains(s, "@") {
		return waTypes.EmptyJID, fmt.Errorf("invalid JID %q: must be in user@server format", s)
	}
	jid, err := waTypes.ParseJID(s)
	if err != nil {
		return waTypes.EmptyJID, fmt.Errorf("invalid JID %q: %w", s, err)
	}
	return jid, nil
}

// isGroup returns true if the JID is a group JID.
func isGroup(jid waTypes.JID) bool {
	return jid.Server == waTypes.GroupServer
}

// FormatRecipient takes a phone number or JID string and returns a proper
// whatsmeow JID. If the input already contains '@', it is parsed as-is;
// otherwise '@s.whatsapp.net' is appended. A leading '+' is stripped,
// since WhatsApp JIDs use bare E.164 digits (e.g. "250725258789"),
// not the "+250725258789" display form.
func FormatRecipient(to string) waTypes.JID {
	to = strings.TrimPrefix(to, "+")
	for _, c := range to {
		if c == '@' {
			jid, err := waTypes.ParseJID(to)
			if err != nil {
				return waTypes.NewJID(to, waTypes.DefaultUserServer)
			}
			return jid
		}
	}
	return waTypes.NewJID(to, waTypes.DefaultUserServer)
}

// CleanJID returns a copy of the JID with Device set to 0.
func CleanJID(jid waTypes.JID) waTypes.JID {
	jid.Device = 0
	return jid
}
