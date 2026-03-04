package identity

import (
	"context"

	"go.mau.fi/whatsmeow/store"
	waTypes "go.mau.fi/whatsmeow/types"
)

// Identity represents a WhatsApp user with all known identifiers.
// The "id" field is always set: prefers phone-based JID, falls back to LID.
type Identity struct {
	ID     string `json:"id"`
	LID    string `json:"lid,omitempty"`
	Phone  string `json:"phone,omitempty"`
	Device uint16 `json:"device,omitempty"`
}

// Sender extends Identity with an IsMe flag for message/event origins.
type Sender struct {
	Identity
	IsMe bool `json:"isMe"`
}

// Resolve builds an Identity from a primary JID and an optional alternate JID,
// using the LID store to fill in missing sides when available.
func Resolve(ctx context.Context, jid, altJID waTypes.JID, lids store.LIDStore) Identity {
	var phone, lid string
	var device uint16

	// Extract device from the primary JID.
	if jid.Device != 0 {
		device = jid.Device
	}

	// Categorize primary JID.
	categorize(jid, &phone, &lid)

	// Categorize alt JID.
	categorize(altJID, &phone, &lid)

	// If one side is still missing, try the LID store.
	if lids != nil {
		if phone == "" && lid != "" {
			lidJID := waTypes.JID{User: lid, Server: waTypes.HiddenUserServer}
			if pn, err := lids.GetPNForLID(ctx, lidJID); err == nil && pn.User != "" {
				phone = pn.User
			}
		} else if lid == "" && phone != "" {
			pnJID := waTypes.JID{User: phone, Server: waTypes.DefaultUserServer}
			if l, err := lids.GetLIDForPN(ctx, pnJID); err == nil && l.User != "" {
				lid = l.User
			}
		}
	}

	// Build the primary ID: prefer phone-based JID, fall back to LID.
	var id string
	if phone != "" {
		id = phone + "@" + waTypes.DefaultUserServer
	} else if lid != "" {
		id = lid + "@" + waTypes.HiddenUserServer
	} else {
		// Fallback: use the original JID string (e.g. for group server JIDs).
		id = cleanJID(jid).String()
	}

	return Identity{
		ID:     id,
		LID:    formatLID(lid),
		Phone:  phone,
		Device: device,
	}
}

// ResolveSender builds a Sender from a JID with an IsMe flag.
func ResolveSender(ctx context.Context, jid waTypes.JID, isMe bool, altJID waTypes.JID, lids store.LIDStore) Sender {
	return Sender{
		Identity: Resolve(ctx, jid, altJID, lids),
		IsMe:     isMe,
	}
}

// categorize sets phone or lid based on the JID's server.
func categorize(jid waTypes.JID, phone, lid *string) {
	if jid.User == "" {
		return
	}
	switch jid.Server {
	case waTypes.DefaultUserServer:
		if *phone == "" {
			*phone = jid.User
		}
	case waTypes.HiddenUserServer:
		if *lid == "" {
			*lid = jid.User
		}
	}
}

// cleanJID returns a copy of the JID with Device set to 0.
func cleanJID(jid waTypes.JID) waTypes.JID {
	return waTypes.JID{
		User:   jid.User,
		Server: jid.Server,
	}
}

// formatLID returns the full LID string (user@lid) or empty if user is empty.
func formatLID(user string) string {
	if user == "" {
		return ""
	}
	return user + "@" + waTypes.HiddenUserServer
}
