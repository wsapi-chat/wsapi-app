package event

import (
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectorContext holds the dependencies needed by projector functions.
type ProjectorContext struct {
	Client     *whatsmeow.Client
	InstanceID string
}

// LIDs returns the LID store from the client, or nil if unavailable.
func (pctx *ProjectorContext) LIDs() store.LIDStore {
	if pctx != nil && pctx.Client != nil && pctx.Client.Store != nil {
		return pctx.Client.Store.LIDs
	}
	return nil
}

// Project takes a raw whatsmeow event and returns a typed Event envelope.
// The bool return indicates whether the event should be published (false = discard).
func Project(evt interface{}, pctx *ProjectorContext) (Event, bool) {
	var (
		eventType string
		data      any
		publish   bool
	)

	switch e := evt.(type) {
	// --- Messages ---
	case *waEvents.Message:
		eventType, data, publish = ProjectMessage(e, pctx)

	// --- Receipts ---
	case *waEvents.Receipt:
		eventType, data, publish = ProjectReceipt(e, pctx)

	// --- Message actions ---
	case *waEvents.DeleteForMe:
		eventType, data, publish = ProjectMessageDeleteForMe(e, pctx)
	case *waEvents.Star:
		eventType, data, publish = ProjectStar(e, pctx)

	// --- Session ---
	case *waEvents.PairSuccess:
		eventType, data, publish = ProjectPairSuccess(e)
	case *waEvents.PairError:
		eventType, data, publish = ProjectPairError(e)
	case *waEvents.LoggedOut:
		eventType, data, publish = ProjectLoggedOut(e)

	// --- Chat settings ---
	case *waEvents.Mute:
		eventType, data, publish = ProjectMute(e, pctx)
	case *waEvents.Pin:
		eventType, data, publish = ProjectPin(e, pctx)
	case *waEvents.Archive:
		eventType, data, publish = ProjectArchive(e, pctx)
	case *waEvents.MarkChatAsRead:
		eventType, data, publish = ProjectMarkChatAsRead(e, pctx)

	// --- Chat events ---
	case *waEvents.ChatPresence:
		eventType, data, publish = ProjectChatPresence(e, pctx)
	case *waEvents.PushName:
		eventType, data, publish = ProjectPushName(e, pctx)
	case *waEvents.BusinessName:
		eventType, data, publish = ProjectBusinessName(e)
	case *waEvents.Picture:
		eventType, data, publish = ProjectPicture(e, pctx)
	case *waEvents.Presence:
		eventType, data, publish = ProjectPresence(e, pctx)
	case *waEvents.UserAbout:
		eventType, data, publish = ProjectUserAbout(e, pctx)

	// --- Groups ---
	case *waEvents.GroupInfo:
		eventType, data, publish = ProjectGroupInfo(e, pctx)
	case *waEvents.JoinedGroup:
		eventType, data, publish = ProjectJoinedGroup(e, pctx)

	// --- Newsletters ---
	case *waEvents.NewsletterJoin:
		eventType, data, publish = ProjectNewsletterJoin(e, pctx)
	case *waEvents.NewsletterLeave:
		eventType, data, publish = ProjectNewsletterLeave(e, pctx)
	case *waEvents.NewsletterMuteChange:
		eventType, data, publish = ProjectNewsletterMuteChange(e, pctx)

	// --- Contacts ---
	case *waEvents.Contact:
		eventType, data, publish = ProjectContact(e, pctx)

	// --- Calls ---
	case *waEvents.CallOffer:
		eventType, data, publish = ProjectCallOffer(e, pctx)
	case *waEvents.CallTerminate:
		eventType, data, publish = ProjectCallTerminated(e, pctx)
	case *waEvents.CallAccept:
		eventType, data, publish = ProjectCallAccepted(e, pctx)

	// --- History sync ---
	case *waEvents.HistorySync:
		eventType, data, publish = ProjectHistorySync(e, pctx)

	default:
		// Unknown/unmapped event type
		return Event{}, false
	}

	if !publish || data == nil {
		return Event{}, false
	}

	return NewEvent(pctx.InstanceID, eventType, data), true
}

// ProjectContact converts a whatsmeow Contact event into a ContactEvent.
func ProjectContact(evt *waEvents.Contact, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeContact, nil, false
	}

	// Discard full-sync events
	if evt.FromFullSync {
		return TypeContact, nil, false
	}

	data := ContactEvent{
		Contact:            resolveIdentity(evt.JID, pctx),
		FullName:           evt.Action.GetFullName(),
		InPhoneAddressBook: evt.Action.GetSaveOnPrimaryAddressbook(),
	}

	return TypeContact, data, true
}
