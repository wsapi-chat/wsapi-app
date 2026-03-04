package event

import (
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectMute converts a whatsmeow Mute event into a ChatSettingEvent.
func ProjectMute(evt *waEvents.Mute, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatSetting, nil, false
	}

	// Discard full-sync events
	if evt.FromFullSync {
		return TypeChatSetting, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)

	muteSetting := &ChatMuteSetting{
		IsMuted: evt.Action.GetMuted(),
	}

	muteEndTimestamp := evt.Action.GetMuteEndTimestamp()
	if muteEndTimestamp > 0 {
		muteSetting.MuteEndTime = UnixToRFC3339(muteEndTimestamp)
	}

	data := ChatSettingEvent{
		ID:          chatID,
		SettingType: "mute",
		Mute:        muteSetting,
	}

	return TypeChatSetting, data, true
}

// ProjectPin converts a whatsmeow Pin event into a ChatSettingEvent.
func ProjectPin(evt *waEvents.Pin, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatSetting, nil, false
	}

	// Discard full-sync events
	if evt.FromFullSync {
		return TypeChatSetting, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)

	data := ChatSettingEvent{
		ID:          chatID,
		SettingType: "pin",
		Pin: &ChatPinSetting{
			IsPinned: evt.Action.GetPinned(),
		},
	}

	return TypeChatSetting, data, true
}

// ProjectArchive converts a whatsmeow Archive event into a ChatSettingEvent.
func ProjectArchive(evt *waEvents.Archive, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatSetting, nil, false
	}

	// Discard full-sync events
	if evt.FromFullSync {
		return TypeChatSetting, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)

	data := ChatSettingEvent{
		ID:          chatID,
		SettingType: "archive",
		Archive: &ChatArchiveSetting{
			IsArchived: evt.Action.GetArchived(),
		},
	}

	return TypeChatSetting, data, true
}

// ProjectMarkChatAsRead converts a whatsmeow MarkChatAsRead event into a ChatSettingEvent.
func ProjectMarkChatAsRead(evt *waEvents.MarkChatAsRead, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatSetting, nil, false
	}

	// Discard full-sync events
	if evt.FromFullSync {
		return TypeChatSetting, nil, false
	}

	data := ChatSettingEvent{
		ID:          projectChatID(evt.JID, pctx),
		SettingType: "read",
		Read: &ChatReadSetting{
			IsRead: evt.Action.GetRead(),
		},
	}

	return TypeChatSetting, data, true
}

// ProjectChatPresence converts a whatsmeow ChatPresence event into a ChatPresenceEvent.
func ProjectChatPresence(evt *waEvents.ChatPresence, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatPresence, nil, false
	}

	var state string
	switch evt.State {
	case waTypes.ChatPresenceComposing:
		if evt.Media == waTypes.ChatPresenceMediaAudio {
			state = "recording"
		} else {
			state = "typing"
		}
	case waTypes.ChatPresencePaused:
		state = "paused"
	}

	data := ChatPresenceEvent{
		ID:     projectChatID(evt.Chat, pctx),
		Sender: resolveSender(evt.Sender, evt.IsFromMe, evt.SenderAlt, pctx),
		State:  state,
	}

	return TypeChatPresence, data, true
}

// ProjectPushName converts a whatsmeow PushName event into a ChatPushNameEvent.
func ProjectPushName(evt *waEvents.PushName, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatPushName, nil, false
	}

	user := resolveIdentity(evt.JID, pctx)
	if user.ID == "" {
		return TypeChatPushName, nil, false
	}

	data := ChatPushNameEvent{
		User:     user,
		PushName: evt.NewPushName,
	}

	return TypeChatPushName, data, true
}

// ProjectBusinessName handles a whatsmeow BusinessName event.
// In v2 this is a discard-only event (no WSAPIStore persistence).
func ProjectBusinessName(evt *waEvents.BusinessName) (string, any, bool) {
	// BusinessName events were only used for store persistence in v1.
	// Without WSAPIStore we discard them.
	return TypeChatPushName, nil, false
}

// ProjectPicture converts a whatsmeow Picture event into a ChatPictureEvent.
func ProjectPicture(evt *waEvents.Picture, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatPicture, nil, false
	}

	chatID := projectChatID(evt.JID, pctx)
	if chatID == "" {
		return TypeChatPicture, nil, false
	}

	isMe := false
	if pctx != nil && pctx.Client != nil && pctx.Client.Store != nil && pctx.Client.Store.ID != nil {
		isMe = evt.Author.User == pctx.Client.Store.ID.User
	}

	data := ChatPictureEvent{
		ID:        chatID,
		Sender:    resolveSender(evt.Author, isMe, waTypes.EmptyJID, pctx),
		PictureID: evt.PictureID,
	}

	return TypeChatPicture, data, true
}

// ProjectPresence converts a whatsmeow Presence event into a PresenceEvent.
func ProjectPresence(evt *waEvents.Presence, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeUserPresence, nil, false
	}

	status := "available"
	if evt.Unavailable {
		status = "unavailable"
	}

	data := PresenceEvent{
		User:   resolveIdentity(evt.From, pctx),
		Status: status,
	}

	if !evt.LastSeen.IsZero() {
		data.LastSeen = evt.LastSeen
	}

	return TypeUserPresence, data, true
}

// ProjectUserAbout converts a whatsmeow UserAbout event into a ChatStatusEvent.
func ProjectUserAbout(evt *waEvents.UserAbout, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeChatStatus, nil, false
	}

	user := resolveIdentity(evt.JID, pctx)
	if user.ID == "" {
		return TypeChatStatus, nil, false
	}

	data := ChatStatusEvent{
		User:   user,
		Status: evt.Status,
	}

	return TypeChatStatus, data, true
}
