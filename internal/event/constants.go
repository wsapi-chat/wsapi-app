package event

const (
	TypeLoggedOut = "logged_out"
	TypeLoggedIn  = "logged_in"
	TypeLoginError = "login_error"

	TypeInitialSyncFinished = "initial_sync_finished"

	TypeChatSetting  = "chat_setting"
	TypeChatPresence = "chat_presence"
	TypeChatPushName = "chat_push_name"
	TypeChatStatus   = "chat_status"
	TypeChatPicture  = "chat_picture"

	TypeMessage            = "message"
	TypeMessageRead        = "message_read"
	TypeMessageDelete      = "message_delete"
	TypeMessageStar        = "message_star"
	TypeMessageHistorySync = "message_history_sync"

	TypeContact    = "contact"
	TypeGroup      = "group"
	TypeNewsletter = "newsletter"

	TypeCallOffer     = "call_offer"
	TypeCallTerminate = "call_terminate"
	TypeCallAccept    = "call_accept"

	TypeUserPresence = "user_presence"
)

// systemEvents are lifecycle events that are always delivered regardless of
// event filters. They must not appear in user-configured filter lists.
var systemEvents = map[string]bool{
	TypeLoggedIn:            true,
	TypeLoggedOut:           true,
	TypeLoginError:          true,
	TypeInitialSyncFinished: true,
}

// IsSystemEvent reports whether the event type is a lifecycle event that is
// always delivered and cannot be filtered.
func IsSystemEvent(eventType string) bool {
	return systemEvents[eventType]
}

// StripSystemEvents returns a copy of filters with any system events removed.
func StripSystemEvents(filters []string) []string {
	out := make([]string, 0, len(filters))
	for _, f := range filters {
		if !systemEvents[f] {
			out = append(out, f)
		}
	}
	return out
}
