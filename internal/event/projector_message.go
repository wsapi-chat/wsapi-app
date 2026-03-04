package event

import (
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
)

// ProjectMessage converts a whatsmeow Message event into a MessageEvent.
// Returns (eventType, data, publish). When publish is false the event should be discarded.
func ProjectMessage(msg *waEvents.Message, pctx *ProjectorContext) (string, any, bool) {
	if msg == nil {
		return TypeMessage, nil, false
	}

	// Discard peer-category messages
	if msg.Info.Category == "peer" {
		return TypeMessage, nil, false
	}

	if msg.Info.ID == "" {
		return TypeMessage, nil, false
	}

	result := MessageEvent{
		ID:      msg.Info.ID,
		Time:    msg.Info.Timestamp,
		ChatID:  projectChatID(msg.Info.Chat, pctx),
		Sender:  resolveSender(msg.Info.Sender, msg.Info.IsFromMe, msg.Info.SenderAlt, pctx),
		IsGroup: msg.Info.IsGroup,
	}

	// Determine initial type from message info
	if msg.Info.Type != "" {
		result.Type = msg.Info.Type
	} else {
		result.Type = "unknown"
	}

	// Status broadcast check
	if msg.Info.Chat == waTypes.StatusBroadcastJID {
		result.IsStatus = true
	}

	// Message content
	if msg.Message != nil {
		// Determine message type and extract content
		if conversationText := msg.Message.GetConversation(); conversationText != "" {
			result.Type = "text"
			result.Text = conversationText
		} else if imgMsg := msg.Message.GetImageMessage(); imgMsg != nil {
			projectImageMessage(imgMsg, &result, pctx)
		} else if videoMsg := msg.Message.GetVideoMessage(); videoMsg != nil {
			projectVideoMessage(videoMsg, &result, pctx)
		} else if audioMsg := msg.Message.GetAudioMessage(); audioMsg != nil {
			projectAudioMessage(audioMsg, &result, pctx)
		} else if docMsg := msg.Message.GetDocumentMessage(); docMsg != nil {
			projectDocumentMessage(docMsg, &result, pctx)
		} else if stickerMsg := msg.Message.GetStickerMessage(); stickerMsg != nil {
			projectStickerMessage(stickerMsg, &result, pctx)
		} else if reactionMsg := msg.Message.GetReactionMessage(); reactionMsg != nil {
			projectReactionMessage(reactionMsg, &result)
		} else if contactMsg := msg.Message.GetContactMessage(); contactMsg != nil {
			projectContactMessage(contactMsg, &result)
		} else if contactArrayMsg := msg.Message.GetContactsArrayMessage(); contactArrayMsg != nil {
			projectContactArrayMessage(contactArrayMsg, &result)
		} else if extMsg := msg.Message.GetExtendedTextMessage(); extMsg != nil {
			projectExtendedTextMessage(extMsg, &result, pctx)
		} else if pinInChatMsg := msg.Message.GetPinInChatMessage(); pinInChatMsg != nil {
			messageContextInfo := msg.Message.GetMessageContextInfo()
			projectPinInChatMessage(pinInChatMsg, messageContextInfo, &result)
		} else if protoMsg := msg.Message.GetProtocolMessage(); protoMsg != nil {
			if protoMsg.Type != nil {
				switch *protoMsg.Type {
				case waE2E.ProtocolMessage_MESSAGE_EDIT:
					projectEditMessage(protoMsg, &result)
				case waE2E.ProtocolMessage_REVOKE:
					return projectMessageDeleteForAll(msg, pctx)
				case waE2E.ProtocolMessage_EPHEMERAL_SETTING:
					return projectMessageEphemeralSetting(msg, pctx)
				}
			}
		} else if senderKeyDist := msg.Message.GetSenderKeyDistributionMessage(); senderKeyDist != nil {
			// Sender key distribution message - discard
			return TypeMessage, nil, false
		}
	}

	return TypeMessage, result, true
}

// ProjectReceipt converts a whatsmeow Receipt event into a ReceiptEvent.
func ProjectReceipt(evt *waEvents.Receipt, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeMessageRead, nil, false
	}

	receiptType := mapReceiptType(evt.Type)

	// Discard peerMsg receipts
	if receiptType == "peerMsg" {
		return TypeMessageRead, nil, false
	}

	data := ReceiptEvent{
		ChatID:      projectChatID(evt.Chat, pctx),
		Sender:      resolveSender(evt.Sender, evt.IsFromMe, evt.SenderAlt, pctx),
		IsGroup:     evt.IsGroup,
		Time:        evt.Timestamp,
		ReceiptType: receiptType,
	}

	if evt.MessageSender.User != "" {
		ms := resolveSender(evt.MessageSender, false, waTypes.EmptyJID, pctx)
		data.MessageSender = &ms
	}

	if len(evt.MessageIDs) > 0 {
		data.MessageIDs = evt.MessageIDs
	}

	return TypeMessageRead, data, true
}

// ProjectMessageDeleteForAll converts a revoke protocol message into a MessageDeleteEvent.
func ProjectMessageDeleteForAll(msg *waEvents.Message, pctx *ProjectorContext) (string, any, bool) {
	return projectMessageDeleteForAll(msg, pctx)
}

func projectMessageDeleteForAll(msg *waEvents.Message, pctx *ProjectorContext) (string, any, bool) {
	if msg == nil {
		return TypeMessageDelete, nil, false
	}

	data := MessageDeleteEvent{
		ID:              *msg.Message.ProtocolMessage.Key.ID,
		Sender:          resolveSender(msg.Info.Sender, msg.Info.IsFromMe, msg.Info.SenderAlt, pctx),
		ChatID:          projectChatID(msg.Info.Chat, pctx),
		Time:            msg.Info.Timestamp,
		IsDeletedForAll: true,
	}

	if msg.Info.Chat == waTypes.StatusBroadcastJID {
		data.IsStatus = true
	}

	return TypeMessageDelete, data, true
}

// ProjectMessageDeleteForMe converts a whatsmeow DeleteForMe event into a MessageDeleteEvent.
func ProjectMessageDeleteForMe(evt *waEvents.DeleteForMe, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeMessageDelete, nil, false
	}

	data := MessageDeleteEvent{
		ID:             evt.MessageID,
		ChatID:         projectChatID(evt.ChatJID, pctx),
		Sender:         resolveSender(evt.SenderJID, evt.IsFromMe, waTypes.EmptyJID, pctx),
		Time:           evt.Timestamp,
		IsDeletedForMe: true,
	}

	if evt.ChatJID == waTypes.StatusBroadcastJID {
		data.IsStatus = true
	}

	return TypeMessageDelete, data, true
}

// ProjectStar converts a whatsmeow Star event into a MessageStarEvent.
func ProjectStar(evt *waEvents.Star, pctx *ProjectorContext) (string, any, bool) {
	if evt == nil {
		return TypeMessageStar, nil, false
	}

	data := MessageStarEvent{
		ID:        evt.MessageID,
		ChatID:    projectChatID(evt.ChatJID, pctx),
		Sender:    resolveSender(evt.SenderJID, evt.IsFromMe, waTypes.EmptyJID, pctx),
		Time:      evt.Timestamp,
		IsStarred: evt.Action.GetStarred(),
	}

	return TypeMessageStar, data, true
}

// projectMessageEphemeralSetting handles EPHEMERAL_SETTING protocol messages.
func projectMessageEphemeralSetting(msg *waEvents.Message, pctx *ProjectorContext) (string, any, bool) {
	if msg == nil {
		return TypeChatSetting, nil, false
	}

	chatID := projectChatID(msg.Info.Chat, pctx)

	var expirationSeconds uint32
	if protoMsg := msg.Message.GetProtocolMessage(); protoMsg != nil && protoMsg.EphemeralExpiration != nil {
		expirationSeconds = *protoMsg.EphemeralExpiration
	}

	data := ChatSettingEvent{
		ID:          chatID,
		SettingType: "ephemeral",
		Ephemeral: &ChatEphemeralSetting{
			Expiration: GetEphemeralExpirationString(expirationSeconds),
			Sender:     resolveSender(msg.Info.Sender, msg.Info.IsFromMe, msg.Info.SenderAlt, pctx),
		},
	}

	return TypeChatSetting, data, true
}

// --- Sub-projectors for message content types ---

func projectImageMessage(m *waE2E.ImageMessage, result *MessageEvent, pctx *ProjectorContext) {
	media := &MediaInfo{
		MediaType: "image",
	}
	if m.Mimetype != nil {
		media.MimeType = *m.Mimetype
	}
	if m.FileLength != nil {
		media.FileLength = *m.FileLength
	}
	if m.Caption != nil {
		media.Caption = *m.Caption
	}
	if m.Height != nil {
		media.Height = *m.Height
	}
	if m.Width != nil {
		media.Width = *m.Width
	}
	if len(m.JPEGThumbnail) > 0 {
		media.JPEGThumbnail = m.JPEGThumbnail
	}

	var fileLength uint64
	if m.FileLength != nil {
		fileLength = *m.FileLength
	}
	if id := createMediaID(
		getStringPtrValue(m.URL),
		getStringPtrValue(m.DirectPath),
		getStringPtrValue(m.Mimetype),
		"",
		"image",
		m.MediaKey, m.FileSHA256, m.FileEncSHA256,
		fileLength,
	); id != "" {
		media.ID = id
	}

	if m.ContextInfo != nil {
		projectMediaContextInfo(m.ContextInfo, result, pctx)
	}
	if m.ViewOnce != nil && *m.ViewOnce {
		result.ViewOnce = true
	}

	result.Media = media
	result.Type = "media"
}

func projectVideoMessage(m *waE2E.VideoMessage, result *MessageEvent, pctx *ProjectorContext) {
	media := &MediaInfo{
		MediaType: "video",
	}
	if m.Mimetype != nil {
		media.MimeType = *m.Mimetype
	}
	if m.FileLength != nil {
		media.FileLength = *m.FileLength
	}
	if m.Caption != nil {
		media.Caption = *m.Caption
	}
	if m.Height != nil {
		media.Height = *m.Height
	}
	if m.Width != nil {
		media.Width = *m.Width
	}
	if len(m.JPEGThumbnail) > 0 {
		media.JPEGThumbnail = m.JPEGThumbnail
	}
	if m.Seconds != nil {
		media.Duration = *m.Seconds
	}

	var fileLength uint64
	if m.FileLength != nil {
		fileLength = *m.FileLength
	}
	if id := createMediaID(
		getStringPtrValue(m.URL),
		getStringPtrValue(m.DirectPath),
		getStringPtrValue(m.Mimetype),
		"",
		"video",
		m.MediaKey, m.FileSHA256, m.FileEncSHA256,
		fileLength,
	); id != "" {
		media.ID = id
	}

	if m.ContextInfo != nil {
		projectMediaContextInfo(m.ContextInfo, result, pctx)
	}
	if m.ViewOnce != nil && *m.ViewOnce {
		result.ViewOnce = true
	}

	result.Media = media
	result.Type = "media"
}

func projectAudioMessage(m *waE2E.AudioMessage, result *MessageEvent, pctx *ProjectorContext) {
	mediaType := "audio"
	if m.PTT != nil && *m.PTT {
		mediaType = "voice"
	}

	media := &MediaInfo{
		MediaType: mediaType,
	}
	if m.Mimetype != nil {
		media.MimeType = *m.Mimetype
	}
	if m.FileLength != nil {
		media.FileLength = *m.FileLength
	}
	if m.Seconds != nil {
		media.Duration = *m.Seconds
	}

	mediaTypeStr := "audio"
	if m.PTT != nil && *m.PTT {
		mediaTypeStr = "voice"
	}
	var fileLength uint64
	if m.FileLength != nil {
		fileLength = *m.FileLength
	}
	if id := createMediaID(
		getStringPtrValue(m.URL),
		getStringPtrValue(m.DirectPath),
		getStringPtrValue(m.Mimetype),
		"",
		mediaTypeStr,
		m.MediaKey, m.FileSHA256, m.FileEncSHA256,
		fileLength,
	); id != "" {
		media.ID = id
	}

	if m.ContextInfo != nil {
		projectMediaContextInfo(m.ContextInfo, result, pctx)
	}

	result.Media = media
	result.Type = "media"
}

func projectDocumentMessage(m *waE2E.DocumentMessage, result *MessageEvent, pctx *ProjectorContext) {
	media := &MediaInfo{
		MediaType: "document",
	}
	if m.Mimetype != nil {
		media.MimeType = *m.Mimetype
	}
	if m.FileLength != nil {
		media.FileLength = *m.FileLength
	}
	if m.FileName != nil {
		media.Filename = *m.FileName
	}
	if m.PageCount != nil {
		media.PageCount = *m.PageCount
	}
	if m.Caption != nil {
		media.Caption = *m.Caption
	}
	if len(m.JPEGThumbnail) > 0 {
		media.JPEGThumbnail = m.JPEGThumbnail
	}
	if m.Title != nil {
		media.Title = *m.Title
	}

	var fileLength uint64
	if m.FileLength != nil {
		fileLength = *m.FileLength
	}
	if id := createMediaID(
		getStringPtrValue(m.URL),
		getStringPtrValue(m.DirectPath),
		getStringPtrValue(m.Mimetype),
		getStringPtrValue(m.FileName),
		"document",
		m.MediaKey, m.FileSHA256, m.FileEncSHA256,
		fileLength,
	); id != "" {
		media.ID = id
	}

	if m.ContextInfo != nil {
		projectMediaContextInfo(m.ContextInfo, result, pctx)
	}

	result.Media = media
	result.Type = "media"
}

func projectStickerMessage(m *waE2E.StickerMessage, result *MessageEvent, pctx *ProjectorContext) {
	media := &MediaInfo{
		MediaType: "sticker",
	}
	if m.Mimetype != nil {
		media.MimeType = *m.Mimetype
	}
	if m.FileLength != nil {
		media.FileLength = *m.FileLength
	}
	if m.Height != nil {
		media.Height = *m.Height
	}
	if m.Width != nil {
		media.Width = *m.Width
	}

	var fileLength uint64
	if m.FileLength != nil {
		fileLength = *m.FileLength
	}
	if id := createMediaID(
		getStringPtrValue(m.URL),
		getStringPtrValue(m.DirectPath),
		getStringPtrValue(m.Mimetype),
		"",
		"sticker",
		m.MediaKey, m.FileSHA256, m.FileEncSHA256,
		fileLength,
	); id != "" {
		media.ID = id
	}

	if m.ContextInfo != nil {
		projectMediaContextInfo(m.ContextInfo, result, pctx)
	}

	result.Media = media
	result.Type = "media"
}

func projectMediaContextInfo(contextInfo *waE2E.ContextInfo, result *MessageEvent, pctx *ProjectorContext) {
	if contextInfo == nil {
		return
	}

	// Forwarded flag
	if contextInfo.IsForwarded != nil && *contextInfo.IsForwarded {
		result.IsForwarded = true
	}

	// Mentions
	if len(contextInfo.MentionedJID) > 0 {
		result.Mentions = resolveMentions(contextInfo.MentionedJID, pctx)
	}

	// Reply information
	hasStanzaID := contextInfo.StanzaID != nil && *contextInfo.StanzaID != ""
	hasParticipant := contextInfo.Participant != nil && *contextInfo.Participant != ""

	if hasStanzaID || hasParticipant {
		replyTo := &ReplyInfo{}

		if hasStanzaID {
			replyTo.ID = *contextInfo.StanzaID
		}

		if hasParticipant {
			if jid, err := waTypes.ParseJID(*contextInfo.Participant); err == nil {
				s := resolveSender(jid, false, waTypes.EmptyJID, pctx)
				replyTo.Sender = &s
			}
		}

		if contextInfo.IsForwarded != nil && *contextInfo.IsForwarded {
			replyTo.IsForwarded = true
		}

		// Reply content
		if quotedMsg := contextInfo.GetQuotedMessage(); quotedMsg != nil {
			if quotedMsg.GetConversation() != "" {
				replyTo.Text = quotedMsg.GetConversation()
			} else if quotedMsg.GetExtendedTextMessage() != nil {
				extText := quotedMsg.GetExtendedTextMessage()
				if extText.Text != nil && *extText.Text != "" {
					replyTo.Text = *extText.Text
				}
			}
		}

		result.ReplyTo = replyTo
	}
}

func projectReactionMessage(reactionMsg *waE2E.ReactionMessage, result *MessageEvent) {
	reaction := &ReactionInfo{}
	if reactionMsg.Key != nil {
		if reactionMsg.Key.ID != nil {
			reaction.MessageID = *reactionMsg.Key.ID
		}
	}
	if reactionMsg.Text != nil {
		reaction.Emoji = *reactionMsg.Text
	}
	result.Reaction = reaction
	result.Type = "reaction"
}

func projectContactMessage(contactMsg *waE2E.ContactMessage, result *MessageEvent) {
	if contactMsg.Vcard != nil {
		result.Contact = *contactMsg.Vcard
	}
	result.Type = "contact"
}

func projectContactArrayMessage(contactArrayMsg *waE2E.ContactsArrayMessage, result *MessageEvent) {
	if contactArrayMsg.Contacts != nil {
		contactVCards := make([]string, 0, len(contactArrayMsg.Contacts))
		for _, contactMsg := range contactArrayMsg.Contacts {
			if contactMsg.Vcard != nil {
				contactVCards = append(contactVCards, *contactMsg.Vcard)
			}
		}
		result.ContactArray = contactVCards
	}
	result.Type = "contactArray"
}

func projectExtendedTextMessage(extMsg *waE2E.ExtendedTextMessage, result *MessageEvent, pctx *ProjectorContext) {
	if extMsg == nil {
		return
	}

	// Extended text message
	if extMsg.Text != nil && *extMsg.Text != "" {
		result.Text = *extMsg.Text
		result.Type = "text"
	}

	// Rich text properties
	hasDescription := extMsg.Description != nil && *extMsg.Description != ""
	hasTitle := extMsg.Title != nil && *extMsg.Title != ""
	hasMatchedText := extMsg.MatchedText != nil && *extMsg.MatchedText != ""
	hasThumbnail := len(extMsg.JPEGThumbnail) > 0

	if hasDescription || hasTitle || hasThumbnail {
		extendedText := &ExtendedText{}

		if hasMatchedText {
			extendedText.MatchedText = *extMsg.MatchedText
		}
		if hasDescription {
			extendedText.Description = *extMsg.Description
		}
		if hasTitle {
			extendedText.Title = *extMsg.Title
		}
		if hasThumbnail {
			extendedText.JPEGThumbnail = extMsg.JPEGThumbnail
		}

		result.ExtendedText = extendedText
	}

	// Context info for extended text message
	if contextInfo := extMsg.ContextInfo; contextInfo != nil {
		// Forwarded flag
		if contextInfo.IsForwarded != nil && *contextInfo.IsForwarded {
			result.IsForwarded = true
		}

		// Mentions
		if len(contextInfo.MentionedJID) > 0 {
			result.Mentions = resolveMentions(contextInfo.MentionedJID, pctx)
		}

		// Reply information
		hasStanzaID := contextInfo.StanzaID != nil && *contextInfo.StanzaID != ""
		hasParticipant := contextInfo.Participant != nil && *contextInfo.Participant != ""

		if hasStanzaID || hasParticipant {
			replyTo := &ReplyInfo{}

			if hasStanzaID {
				replyTo.ID = *contextInfo.StanzaID
			}

			if hasParticipant {
				if jid, err := waTypes.ParseJID(*contextInfo.Participant); err == nil {
					s := resolveSender(jid, false, waTypes.EmptyJID, pctx)
					replyTo.Sender = &s
				}
			}

			if contextInfo.IsForwarded != nil && *contextInfo.IsForwarded {
				replyTo.IsForwarded = true
			}

			// Reply content
			if quotedMsg := contextInfo.GetQuotedMessage(); quotedMsg != nil {
				if quotedMsg.GetConversation() != "" {
					replyTo.Text = quotedMsg.GetConversation()
				} else if quotedMsg.GetExtendedTextMessage() != nil {
					extText := quotedMsg.GetExtendedTextMessage()
					if extText.Text != nil && *extText.Text != "" {
						replyTo.Text = *extText.Text
					}
				}
			}

			result.ReplyTo = replyTo
		}

		if contextInfo.Expiration != nil {
			result.EphemeralExpiration = GetEphemeralExpirationString(*contextInfo.Expiration)
		}
	}
}

func projectEditMessage(protoMsg *waE2E.ProtocolMessage, result *MessageEvent) {
	editedMsg := protoMsg.GetEditedMessage()
	if editedMsg != nil {
		if editedMsg.GetConversation() != "" {
			result.Text = editedMsg.GetConversation()
			result.Type = "text"
		} else if editedMsg.GetExtendedTextMessage() != nil {
			extText := editedMsg.GetExtendedTextMessage()
			if extText.Text != nil && *extText.Text != "" {
				result.Text = *extText.Text
				result.Type = "text"
			}
		}
	}

	edit := &EditInfo{}

	if protoMsg.TimestampMS != nil {
		edit.OriginalMessageTime = UnixToRFC3339(protoMsg.GetTimestampMS())
	}

	if protoMsg.Key != nil && protoMsg.Key.ID != nil {
		edit.OriginalMessageID = protoMsg.Key.GetID()
	}

	result.IsEdit = true
	result.Edit = edit
}

func projectPinInChatMessage(pinInChatMsg *waE2E.PinInChatMessage, messageContextInfo *waE2E.MessageContextInfo, result *MessageEvent) {
	if pinInChatMsg.Key == nil {
		return
	}

	action := pinInChatMsg.GetType()

	expiration := "off"
	if messageContextInfo != nil {
		if messageContextInfo.MessageAddOnDurationInSecs != nil {
			switch *messageContextInfo.MessageAddOnDurationInSecs {
			case 24 * 60 * 60:
				expiration = "24h"
			case 7 * 24 * 60 * 60:
				expiration = "7d"
			case 30 * 24 * 60 * 60:
				expiration = "30d"
			}
		}
	}

	pin := &PinInfo{
		MessageID:  pinInChatMsg.Key.GetID(),
		Expiration: expiration,
	}

	if action == waE2E.PinInChatMessage_PIN_FOR_ALL {
		pin.Pinned = true
	} else if action == waE2E.PinInChatMessage_UNPIN_FOR_ALL {
		pin.Pinned = false
	}

	result.Pin = pin
	result.Type = "pinInChat"
}

// mapReceiptType converts a whatsmeow ReceiptType to a string.
func mapReceiptType(t waTypes.ReceiptType) string {
	switch t {
	case waTypes.ReceiptTypeDelivered:
		return "delivered"
	case waTypes.ReceiptTypeRead:
		return "read"
	case waTypes.ReceiptTypePlayed:
		return "played"
	case waTypes.ReceiptTypeReadSelf:
		return "readSelf"
	case waTypes.ReceiptTypeSender:
		return "sender"
	case waTypes.ReceiptTypeRetry:
		return "retry"
	case waTypes.ReceiptTypeServerError:
		return "serverError"
	case waTypes.ReceiptTypeInactive:
		return "inactive"
	case waTypes.ReceiptTypePeerMsg:
		return "peerMsg"
	case waTypes.ReceiptTypeHistorySync:
		return "historySync"
	default:
		return string(t)
	}
}
