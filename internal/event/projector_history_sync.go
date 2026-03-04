package event

import (
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectHistorySync converts a whatsmeow HistorySync event into a publishable event.
// Only ON_DEMAND syncs produce a message_history_sync event with parsed messages.
// All other sync types are discarded. The initial_sync_finished event is handled
// separately by the instance manager after history sync caching completes.
func ProjectHistorySync(evt *waEvents.HistorySync, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil || evt.Data == nil || evt.Data.SyncType == nil {
		return TypeMessageHistorySync, nil, false
	}

	// Only process ON_DEMAND history syncs for message projection.
	if *evt.Data.SyncType != waHistorySync.HistorySync_ON_DEMAND {
		return TypeMessageHistorySync, nil, false
	}

	var messages []MessageEvent

	for _, conv := range evt.Data.Conversations {
		if conv == nil {
			continue
		}

		chatJID, err := waTypes.ParseJID(conv.GetID())
		if err != nil {
			continue
		}

		convMessages := conv.GetMessages()
		if len(convMessages) == 0 {
			continue
		}

		for _, msg := range convMessages {
			waMsg, err := pctx.Client.ParseWebMessage(chatJID, msg.GetMessage())
			if err != nil {
				continue
			}

			_, data, publish := ProjectMessage(waMsg, pctx)
			if !publish {
				continue
			}

			msgEvt, ok := data.(MessageEvent)
			if !ok {
				continue
			}

			// Skip unknown type messages
			if msgEvt.Type == "unknown" {
				continue
			}

			messages = append(messages, msgEvt)
		}
	}

	if len(messages) == 0 {
		return TypeMessageHistorySync, nil, false
	}

	// Set ChatID from the first message when all messages share the same chat
	// (typical for ON_DEMAND syncs which target a single conversation).
	chatID := ""
	if len(messages) > 0 {
		chatID = messages[0].ChatID
		for _, m := range messages[1:] {
			if m.ChatID != chatID {
				chatID = ""
				break
			}
		}
	}

	data := HistorySyncEvent{
		ChatID:   chatID,
		Messages: messages,
	}

	return TypeMessageHistorySync, data, true
}
