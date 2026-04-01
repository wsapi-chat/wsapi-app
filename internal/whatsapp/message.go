package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waCommon "go.mau.fi/whatsmeow/proto/waCommon"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	waTypes "go.mau.fi/whatsmeow/types"

	"google.golang.org/protobuf/proto"
)

// SendOptions holds common optional parameters for sending messages.
type SendOptions struct {
	Mentions            []string
	ReplyTo             string
	ReplyToSenderID     string
	IsForwarded         bool
	EphemeralExpiration string
}

// MediaSendOptions extends SendOptions with media-specific fields.
type MediaSendOptions struct {
	SendOptions
	Caption  string
	ViewOnce bool
}

// MessageService wraps the whatsmeow client for message operations.
type MessageService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// SendText sends a text message to the specified recipient.
func (m *MessageService) SendText(ctx context.Context, to, text string, opts SendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	var msg *waE2E.Message
	if contextInfo != nil {
		msg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: contextInfo,
			},
		}
	} else {
		msg = &waE2E.Message{
			Conversation: proto.String(text),
		}
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendLink sends a link preview message to the specified recipient.
func (m *MessageService) SendLink(ctx context.Context, to, text, url, title, description string, jpegThumbnail []byte, opts SendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:          proto.String(text),
			MatchedText:   proto.String(url),
			Title:         proto.String(title),
			Description:   proto.String(description),
			JPEGThumbnail: jpegThumbnail,
			ContextInfo:   contextInfo,
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendImage sends an image message to the specified recipient.
func (m *MessageService) SendImage(ctx context.Context, to string, data []byte, mimeType string, opts MediaSendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(opts.Caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   contextInfo,
			ViewOnce:      proto.Bool(opts.ViewOnce),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendVideo sends a video message to the specified recipient.
func (m *MessageService) SendVideo(ctx context.Context, to string, data []byte, mimeType string, opts MediaSendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaVideo)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(opts.Caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   contextInfo,
			ViewOnce:      proto.Bool(opts.ViewOnce),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendAudio sends an audio message to the specified recipient.
func (m *MessageService) SendAudio(ctx context.Context, to string, data []byte, mimeType string, opts MediaSendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   contextInfo,
			ViewOnce:      proto.Bool(opts.ViewOnce),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendVoice sends a voice (PTT) message to the specified recipient.
func (m *MessageService) SendVoice(ctx context.Context, to string, data []byte, opts MediaSendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("audio/ogg; codecs=opus"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			PTT:           proto.Bool(true),
			ContextInfo:   contextInfo,
			ViewOnce:      proto.Bool(opts.ViewOnce),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendDocument sends a document message to the specified recipient.
func (m *MessageService) SendDocument(ctx context.Context, to string, data []byte, filename string, opts MediaSendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Title:         proto.String(filename),
			Caption:       proto.String(opts.Caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mime.TypeByExtension(filepath.Ext(filename))),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			FileName:      proto.String(filename),
			ContextInfo:   contextInfo,
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendSticker sends a sticker message to the specified recipient.
func (m *MessageService) SendSticker(ctx context.Context, to string, data []byte, isAnimated bool, opts SendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	uploaded, err := m.uploadMedia(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		StickerMessage: &waE2E.StickerMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/webp"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			ContextInfo:   contextInfo,
			IsAnimated:    proto.Bool(isAnimated),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendContact sends a contact card message to the specified recipient.
func (m *MessageService) SendContact(ctx context.Context, to, displayName, vcard string, opts SendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	msg := &waE2E.Message{
		ContactMessage: &waE2E.ContactMessage{
			DisplayName: proto.String(displayName),
			Vcard:       proto.String(vcard),
			ContextInfo: contextInfo,
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendReaction sends a reaction to a message.
func (m *MessageService) SendReaction(ctx context.Context, to, senderID, messageID, reaction string) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	senderJID := waTypes.EmptyJID
	if senderID != "" {
		senderJID, err = parseSender(senderID)
		if err != nil {
			return "", fmt.Errorf("invalid sender JID: %v", err)
		}
	}

	reactionMsg := m.client.BuildReaction(chatJID, senderJID, messageID, reaction)

	resp, err := m.client.SendMessage(ctx, chatJID, reactionMsg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendLocation sends a location message to the specified recipient.
func (m *MessageService) SendLocation(ctx context.Context, to string, lat, lng float64, name, address, url, ephemeralExp string) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return "", fmt.Errorf("invalid latitude or longitude")
	}

	contextInfo := m.buildContextInfo(chatJID, nil, "", "", false, ephemeralExp)

	msg := &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(lat),
			DegreesLongitude: proto.Float64(lng),
			Name:             proto.String(name),
			Address:          proto.String(address),
			URL:              proto.String(url),
			ContextInfo:      contextInfo,
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// EditMessage edits a previously sent message.
func (m *MessageService) EditMessage(ctx context.Context, to, messageID, text string, opts SendOptions) (string, error) {
	chatJID, err := parseChat(to)
	if err != nil {
		return "", err
	}

	contextInfo := m.buildContextInfo(chatJID, opts.Mentions, opts.ReplyTo, opts.ReplyToSenderID, opts.IsForwarded, opts.EphemeralExpiration)

	var innerMsg *waE2E.Message
	if contextInfo != nil {
		innerMsg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: contextInfo,
			},
		}
	} else {
		innerMsg = &waE2E.Message{
			Conversation: proto.String(text),
		}
	}

	editMsg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			EditedMessage: innerMsg,
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(to),
				FromMe:    proto.Bool(true),
				ID:        proto.String(messageID),
			},
			Type:        waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
			TimestampMS: proto.Int64(time.Now().Unix()),
		},
	}

	resp, err := m.client.SendMessage(ctx, chatJID, editMsg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// DeleteMessage revokes (deletes) a message for all participants.
func (m *MessageService) DeleteMessage(ctx context.Context, chatID, senderID, messageID string) error {
	chatJID, err := parseChat(chatID)
	if err != nil {
		return err
	}
	senderJID, err := parseSender(senderID)
	if err != nil {
		return err
	}

	msg := m.client.BuildRevoke(chatJID, senderJID, messageID)

	_, err = m.client.SendMessage(ctx, chatJID, msg)
	return err
}

// DeleteMessageForMe deletes a message only for the current user.
func (m *MessageService) DeleteMessageForMe(ctx context.Context, chatID, senderID string, isFromMe bool, messageID string, timestamp time.Time) error {
	chatJID, err := parseChat(chatID)
	if err != nil {
		return err
	}

	var patch appstate.PatchInfo

	if isGroup(chatJID) && !isFromMe {
		senderJID, err := parseSender(senderID)
		if err != nil {
			return fmt.Errorf("sender is required for group messages that are from others: %v", err)
		}
		patch = buildDeleteForMe(chatID, senderJID.String(), messageID, isFromMe, timestamp)
	} else {
		patch = buildDeleteForMe(chatID, "0", messageID, isFromMe, timestamp)
	}

	err = m.client.SendAppState(context.Background(), patch)
	if err != nil {
		m.logger.Error("failed to delete message for me", "error", err)

		if strings.Contains(err.Error(), "regular_high") {
			return fmt.Errorf("failed to delete message for me (409 conflict)")
		}
		return fmt.Errorf("failed to delete message for me")
	}

	return nil
}

// MarkAsRead marks a message as read.
func (m *MessageService) MarkAsRead(ctx context.Context, chatID, senderID, messageID, receiptType string) error {
	chatJID, err := parseChat(chatID)
	if err != nil {
		return err
	}

	senderJID, err := parseSender(senderID)
	if err != nil {
		return err
	}

	msgIDs := []waTypes.MessageID{waTypes.MessageID(messageID)}

	var receiptTypeEnum waTypes.ReceiptType
	switch receiptType {
	case "delivered":
		receiptTypeEnum = waTypes.ReceiptTypeDelivered
	case "sender":
		receiptTypeEnum = waTypes.ReceiptTypeSender
	case "read":
		receiptTypeEnum = waTypes.ReceiptTypeRead
	case "played":
		receiptTypeEnum = waTypes.ReceiptTypePlayed
	}

	return m.client.MarkRead(ctx, msgIDs, time.Now(), chatJID, senderJID, receiptTypeEnum)
}

// PinMessage pins or unpins a message in a chat.
func (m *MessageService) PinMessage(ctx context.Context, chatID, senderID, messageID string, pinned bool, expiration string) error {
	chatJID, err := parseChat(chatID)
	if err != nil {
		return err
	}

	senderJID, err := parseSender(senderID)
	if err != nil {
		return err
	}

	var pinExpirationInt uint32
	var pinTypeEnum waE2E.PinInChatMessage_Type

	if pinned {
		pinTypeEnum = waE2E.PinInChatMessage_PIN_FOR_ALL
		switch expiration {
		case "24h":
			pinExpirationInt = 24 * 60 * 60
		case "7d":
			pinExpirationInt = 7 * 24 * 60 * 60
		case "30d":
			pinExpirationInt = 30 * 24 * 60 * 60
		}
	} else {
		pinTypeEnum = waE2E.PinInChatMessage_UNPIN_FOR_ALL
	}

	msg := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{
			Key:               m.client.BuildMessageKey(chatJID, senderJID, messageID),
			Type:              pinTypeEnum.Enum(),
			SenderTimestampMS: proto.Int64(time.Now().Unix()),
		},
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageAddOnDurationInSecs: proto.Uint32(pinExpirationInt),
			MessageAddOnExpiryType:     waE2E.MessageContextInfo_STATIC.Enum(),
		},
	}

	_, err = m.client.SendMessage(ctx, chatJID, msg)
	return err
}

// StarMessage stars or unstars a message.
func (m *MessageService) StarMessage(ctx context.Context, chatID, senderID, messageID string, starred bool) error {
	chatJID, err := parseChat(chatID)
	if err != nil {
		return err
	}

	senderJID, err := parseSender(senderID)
	if err != nil {
		return fmt.Errorf("invalid sender JID: %v", err)
	}
	isme := senderJID.User == m.client.Store.ID.User

	var patch appstate.PatchInfo
	if isGroup(chatJID) {
		patch = buildStar(chatID, senderJID.String(), messageID, isme, starred)
	} else {
		patch = buildStar(chatID, "0", messageID, isme, starred)
	}

	if len(patch.Mutations) == 0 {
		return fmt.Errorf("failed to build star request")
	}

	err = m.client.SendAppState(context.Background(), patch)
	if err != nil {
		return fmt.Errorf("failed to send star request: %v", err)
	}

	return nil
}

// ---------- helpers ----------

// buildContextInfo constructs a ContextInfo for a message.
func (m *MessageService) buildContextInfo(chatJID waTypes.JID, mentions []string, replyTo, replyToSenderID string, isForwarded bool, ephemeralExpiration string) *waE2E.ContextInfo {
	contextInfo := &waE2E.ContextInfo{}
	isContextSet := false

	if replyTo != "" {
		contextInfo.StanzaID = proto.String(replyTo)
		if replyToSenderID != "" {
			contextInfo.Participant = proto.String(replyToSenderID)
		} else {
			contextInfo.Participant = proto.String(chatJID.String())
		}
		contextInfo.QuotedMessage = &waE2E.Message{}
		isContextSet = true
	}

	if len(mentions) > 0 {
		contextInfo.MentionedJID = make([]string, len(mentions))
		for i, mention := range mentions {
			mentionJID, err := parseSender(mention)
			if err == nil {
				contextInfo.MentionedJID[i] = mentionJID.String()
			}
		}
		isContextSet = true
	}

	if isForwarded {
		contextInfo.IsForwarded = proto.Bool(true)
		isContextSet = true
	}

	if ephemeralExpiration != "" && ephemeralExpiration != "off" {
		exp := parseEphemeralExpiration(ephemeralExpiration)
		if exp > 0 {
			contextInfo.Expiration = proto.Uint32(exp)
			isContextSet = true
		}
	}

	if !isContextSet {
		return nil
	}
	return contextInfo
}

// parseEphemeralExpiration converts an ephemeral expiration string to seconds.
func parseEphemeralExpiration(expiration string) uint32 {
	switch expiration {
	case "24h":
		return 24 * 60 * 60
	case "7d":
		return 7 * 24 * 60 * 60
	case "90d":
		return 90 * 24 * 60 * 60
	default:
		return 0
	}
}

// uploadMedia uploads media to WhatsApp servers.
func (m *MessageService) uploadMedia(ctx context.Context, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	uploaded, err := m.client.Upload(ctx, data, mediaType)
	if err != nil {
		return whatsmeow.UploadResponse{}, fmt.Errorf("failed to upload media: %v", err)
	}
	return uploaded, nil
}

// buildDeleteForMe builds an app state patch for deleting a message for the current user.
func buildDeleteForMe(chat, sender, messageID string, isFromMe bool, timestamp time.Time) appstate.PatchInfo {
	strIsFromMe := "0"
	if isFromMe {
		strIsFromMe = "1"
	}

	mutationInfo := appstate.MutationInfo{
		Index:   []string{appstate.IndexDeleteMessageForMe, chat, messageID, strIsFromMe, sender},
		Version: 2,
		Value: &waSyncAction.SyncActionValue{
			DeleteMessageForMeAction: &waSyncAction.DeleteMessageForMeAction{
				DeleteMedia:      proto.Bool(false),
				MessageTimestamp: proto.Int64(timestamp.Unix()),
			},
		},
	}

	return appstate.PatchInfo{
		Type:      appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{mutationInfo},
	}
}

// buildStar builds an app state patch for starring or unstarring a message.
func buildStar(chat, sender, messageID string, isFromMe, starred bool) appstate.PatchInfo {
	strIsFromMe := "0"
	if isFromMe {
		strIsFromMe = "1"
	}

	mutationInfo := appstate.MutationInfo{
		Index:   []string{appstate.IndexStar, chat, messageID, strIsFromMe, sender},
		Version: 2,
		Value: &waSyncAction.SyncActionValue{
			StarAction: &waSyncAction.StarAction{
				Starred: &starred,
			},
		},
	}

	return appstate.PatchInfo{
		Type:      appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{mutationInfo},
	}
}
