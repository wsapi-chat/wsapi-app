package whatsapp

import (
	"fmt"

	waTypes "go.mau.fi/whatsmeow/types"
)

// parseJID parses a string JID into a whatsmeow JID.
func parseJID(s string) (waTypes.JID, error) {
	jid, err := waTypes.ParseJID(s)
	if err != nil {
		return waTypes.EmptyJID, fmt.Errorf("invalid JID %q: %w", s, err)
	}
	return jid, nil
}

// parseChat parses a chat JID string. For user JIDs that don't contain a
// server suffix, it appends @s.whatsapp.net.
func parseChat(chat string) (waTypes.JID, error) {
	return FormatRecipient(chat), nil
}

// parseSender parses a sender JID string. Accepts both full JID format
// (e.g. "1234567890@s.whatsapp.net") and phone-only (e.g. "1234567890").
func parseSender(sender string) (waTypes.JID, error) {
	if sender == "" {
		return waTypes.EmptyJID, fmt.Errorf("sender is empty")
	}
	// If it contains '@', validate strictly as a full JID.
	for _, c := range sender {
		if c == '@' {
			return parseJID(sender)
		}
	}
	// Phone-only input: append the default server.
	return waTypes.NewJID(sender, waTypes.DefaultUserServer), nil
}

// isGroup returns true if the JID is a group JID.
func isGroup(jid waTypes.JID) bool {
	return jid.Server == waTypes.GroupServer
}

// FormatRecipient takes a phone number or JID string and returns a proper
// whatsmeow JID. If the input already contains '@', it is parsed as-is;
// otherwise '@s.whatsapp.net' is appended.
func FormatRecipient(to string) waTypes.JID {
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
