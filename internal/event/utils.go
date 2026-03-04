package event

import (
	"context"
	"time"

	waTypes "go.mau.fi/whatsmeow/types"
)

// UnixToRFC3339 converts a unix timestamp in milliseconds to an RFC3339 string.
func UnixToRFC3339(ts int64) string {
	return time.UnixMilli(ts).UTC().Format(time.RFC3339)
}

// projectChatID converts a whatsmeow JID to a clean string for use as a chat identifier.
// Strips the device portion if present. For LID-based JIDs (@lid server), attempts to
// resolve to a phone-based JID via the LID store; falls back to the raw LID string.
func projectChatID(jid waTypes.JID, pctx *ProjectorContext) string {
	clean := jid
	if clean.Device != 0 {
		clean = waTypes.JID{User: clean.User, Server: clean.Server}
	}

	// Resolve LID-based JIDs to phone-based JIDs when possible.
	if clean.Server == waTypes.HiddenUserServer {
		if lids := lidsFromCtx(pctx); lids != nil {
			if pn, err := lids.GetPNForLID(context.Background(), clean); err == nil && pn.User != "" {
				return waTypes.JID{User: pn.User, Server: waTypes.DefaultUserServer}.String()
			}
		}
	}

	return clean.String()
}

// GetEphemeralExpirationString converts an expiration duration in seconds to a
// human-readable string ("off", "24h", "7d", "90d").
func GetEphemeralExpirationString(expiration uint32) string {
	switch expiration {
	case 0:
		return "off"
	case 24 * 60 * 60:
		return "24h"
	case 7 * 24 * 60 * 60:
		return "7d"
	case 90 * 24 * 60 * 60:
		return "90d"
	}
	return "off"
}

// getStringPtrValue safely dereferences a *string, returning "" if nil.
func getStringPtrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
