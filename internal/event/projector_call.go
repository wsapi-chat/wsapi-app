package event

import (
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectCallOffer converts a whatsmeow CallOffer event into a CallOfferEvent.
func ProjectCallOffer(evt *waEvents.CallOffer, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeCallOffer, nil, false
	}

	caller := resolveIdentity(evt.CallCreator, pctx)

	groupJid := ""
	if val, ok := evt.Data.Attrs["group-jid"]; ok && val != nil {
		switch v := val.(type) {
		case string:
			groupJid = v
		case waTypes.JID:
			groupJid = projectChatID(v, pctx)
		}
	}

	chatID := caller.ID
	if groupJid != "" {
		chatID = groupJid
	}

	// Detect video call by looking for a child node with Tag == "video".
	isVideo := false
	if evt.Data != nil {
		for _, child := range evt.Data.GetChildren() {
			if child.Tag == "video" {
				isVideo = true
				break
			}
		}
	}

	data := CallOfferEvent{
		ID:      evt.CallID,
		Caller:  caller,
		ChatID:  chatID,
		IsGroup: groupJid != "",
		Time:    evt.Timestamp,
		IsVideo: isVideo,
	}

	return TypeCallOffer, data, true
}

// ProjectCallTerminated converts a whatsmeow CallTerminate event into a CallTerminateEvent.
func ProjectCallTerminated(evt *waEvents.CallTerminate, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeCallTerminate, nil, false
	}

	var reason string
	switch evt.Reason {
	case "rejected_elsewhere":
		reason = "rejected"
	case "accepted_elsewhere":
		reason = "ended"
	case "group_call_ended":
		reason = "ended"
	default:
		reason = "unknown"
	}

	data := CallTerminateEvent{
		ID:     evt.CallID,
		Caller: resolveIdentity(evt.CallCreator, pctx),
		Time:   evt.Timestamp,
		Reason: reason,
	}

	return TypeCallTerminate, data, true
}

// ProjectCallAccepted converts a whatsmeow CallAccept event into a CallAcceptEvent.
func ProjectCallAccepted(evt *waEvents.CallAccept, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeCallAccept, nil, false
	}

	data := CallAcceptEvent{
		ID:     evt.CallID,
		Caller: resolveIdentity(evt.CallCreator, pctx),
		Time:   evt.Timestamp,
	}

	return TypeCallAccept, data, true
}
