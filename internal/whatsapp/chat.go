package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	waTypes "go.mau.fi/whatsmeow/types"

	"google.golang.org/protobuf/proto"
)

// ChatPictureResponse is the domain response type for a chat picture.
type ChatPictureResponse struct {
	PictureID  string `json:"pictureId"`
	PictureURL string `json:"pictureUrl"`
}

// BusinessCategory represents a business category.
type BusinessCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BusinessHours represents business hours for a day.
type BusinessHours struct {
	DayOfWeek string `json:"dayOfWeek"`
	Mode      string `json:"mode"`
	OpenTime  string `json:"openTime"`
	CloseTime string `json:"closeTime"`
}

// BusinessProfileResponse is the domain response type for a business profile.
type BusinessProfileResponse struct {
	ID                    string             `json:"id"`
	Address               string             `json:"address"`
	Email                 string             `json:"email"`
	Description           string             `json:"description"`
	Website               string             `json:"website"`
	Latitude              float64            `json:"latitude"`
	Longitude             float64            `json:"longitude"`
	MemberSince           string             `json:"memberSince"`
	Categories            []BusinessCategory `json:"categories"`
	BusinessHours         []BusinessHours    `json:"businessHours"`
	BusinessHoursTimeZone string             `json:"businessHoursTimeZone"`
	ProfileOptions        map[string]string  `json:"profileOptions"`
}

// ChatListItem is the response type for a single chat in the list.
type ChatListItem struct {
	ID           string  `json:"id"`
	LID          string  `json:"lid,omitempty"`
	IsGroup      bool    `json:"isGroup"`
	IsArchived   bool    `json:"isArchived"`
	IsPinned     bool    `json:"isPinned"`
	IsMuted      bool    `json:"isMuted"`
	MuteEndTime  *string `json:"muteEndTime,omitempty"`
	PushName     string  `json:"pushName,omitempty"`
	BusinessName string  `json:"businessName,omitempty"`
	FullName     string  `json:"fullName,omitempty"`
	LastActivity *string `json:"lastActivity,omitempty"`
}

// ChatService wraps the whatsmeow client for chat operations.
type ChatService struct {
	client    *whatsmeow.Client
	logger    *slog.Logger
	chatStore *ChatStore
}

func (c *ChatService) getOurJID() string {
	if c.client.Store.ID == nil {
		return ""
	}
	return c.client.Store.ID.String()
}

// getChatSettings looks up chat settings by JID, falling back to the
// alternate identifier (LID ↔ phone JID) when whatsmeow stored them
// under the other form.
func (c *ChatService) getChatSettings(ctx context.Context, jid waTypes.JID) (waTypes.LocalChatSettings, error) {
	settings, err := c.client.Store.ChatSettings.GetChatSettings(ctx, jid)
	if err != nil {
		return settings, err
	}
	if settings.Found {
		return settings, nil
	}

	// Try the alternate identifier.
	lids := c.client.Store.LIDs
	if lids == nil {
		return settings, nil
	}

	var alt waTypes.JID
	switch jid.Server {
	case waTypes.DefaultUserServer:
		alt, _ = lids.GetLIDForPN(ctx, jid)
	case waTypes.HiddenUserServer:
		alt, _ = lids.GetPNForLID(ctx, jid)
	}
	if alt.User != "" {
		settings, err = c.client.Store.ChatSettings.GetChatSettings(ctx, alt)
	}
	return settings, err
}

// GetChatPicture retrieves the profile picture for a chat.
func (c *ChatService) GetChatPicture(ctx context.Context, chatID string) (ChatPictureResponse, error) {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return ChatPictureResponse{}, fmt.Errorf("invalid JID: %w", err)
	}

	picture, err := c.client.GetProfilePictureInfo(ctx, jid, nil)
	if err != nil {
		return ChatPictureResponse{}, fmt.Errorf("failed to get chat picture: %w", err)
	}

	return ChatPictureResponse{
		PictureID:  picture.ID,
		PictureURL: picture.URL,
	}, nil
}

// ListChats returns all known chats for the device, enriched with
// contact info and chat settings from the whatsmeow store.
func (c *ChatService) ListChats(ctx context.Context) ([]ChatListItem, error) {
	ourJID := c.getOurJID()
	if ourJID == "" {
		return nil, fmt.Errorf("device not paired")
	}

	chatRecords, err := c.chatStore.List(ctx, ourJID)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}

	contacts, err := c.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		c.logger.Warn("failed to fetch contacts for chat enrichment", "error", err)
	}

	items := make([]ChatListItem, 0, len(chatRecords))
	for _, rec := range chatRecords {
		item := ChatListItem{
			ID:      rec.ChatJID,
			IsGroup: rec.IsGroup,
		}

		jid, err := waTypes.ParseJID(rec.ChatJID)
		if err == nil {
			if contact, ok := contacts[jid]; ok {
				item.PushName = contact.PushName
				item.BusinessName = contact.BusinessName
				item.FullName = contact.FullName
			}

			// Resolve LID for phone-based JIDs.
			if lids := c.client.Store.LIDs; lids != nil && jid.Server == waTypes.DefaultUserServer {
				if l, lidErr := lids.GetLIDForPN(ctx, jid); lidErr == nil && l.User != "" {
					item.LID = l.User + "@" + waTypes.HiddenUserServer
				}
			}

			settings, settingsErr := c.getChatSettings(ctx, jid)
			if settingsErr == nil && settings.Found {
				item.IsPinned = settings.Pinned
				item.IsArchived = settings.Archived
				item.IsMuted = !settings.MutedUntil.IsZero() && settings.MutedUntil.After(time.Now())
				if item.IsMuted {
					t := settings.MutedUntil.UTC().Format(time.RFC3339)
					item.MuteEndTime = &t
				}
			}
		}

		if !rec.LastActivity.IsZero() {
			t := rec.LastActivity.UTC().Format(time.RFC3339)
			item.LastActivity = &t
		}

		items = append(items, item)
	}
	return items, nil
}

// GetChatInfo returns details for a single chat, enriched with contact info
// and chat settings from the whatsmeow store.
func (c *ChatService) GetChatInfo(ctx context.Context, chatID string) (ChatListItem, error) {
	ourJID := c.getOurJID()
	if ourJID == "" {
		return ChatListItem{}, fmt.Errorf("device not paired")
	}

	rec, err := c.chatStore.Get(ctx, ourJID, chatID)
	if err != nil {
		return ChatListItem{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}

	item := ChatListItem{
		ID:      rec.ChatJID,
		IsGroup: rec.IsGroup,
	}

	jid, err := waTypes.ParseJID(rec.ChatJID)
	if err == nil {
		contact, contactErr := c.client.Store.Contacts.GetContact(ctx, jid)
		if contactErr == nil && contact.Found {
			item.PushName = contact.PushName
			item.BusinessName = contact.BusinessName
			item.FullName = contact.FullName
		}

		// Resolve LID for phone-based JIDs.
		if lids := c.client.Store.LIDs; lids != nil && jid.Server == waTypes.DefaultUserServer {
			if l, lidErr := lids.GetLIDForPN(ctx, jid); lidErr == nil && l.User != "" {
				item.LID = l.User + "@" + waTypes.HiddenUserServer
			}
		}

		settings, settingsErr := c.getChatSettings(ctx, jid)
		if settingsErr == nil && settings.Found {
			item.IsPinned = settings.Pinned
			item.IsArchived = settings.Archived
			item.IsMuted = !settings.MutedUntil.IsZero() && settings.MutedUntil.After(time.Now())
			if item.IsMuted {
				t := settings.MutedUntil.UTC().Format(time.RFC3339)
				item.MuteEndTime = &t
			}
		}
	}

	if !rec.LastActivity.IsZero() {
		t := rec.LastActivity.UTC().Format(time.RFC3339)
		item.LastActivity = &t
	}

	return item, nil
}

// SendChatPresence sends a presence update (typing, paused, recording) to a chat.
func (c *ChatService) SendChatPresence(ctx context.Context, chatID string, state string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	var presenceState waTypes.ChatPresence
	var presenceMedia waTypes.ChatPresenceMedia

	switch state {
	case "typing":
		presenceState = waTypes.ChatPresenceComposing
	case "paused":
		presenceState = waTypes.ChatPresencePaused
	case "recording":
		presenceState = waTypes.ChatPresenceComposing
		presenceMedia = waTypes.ChatPresenceMediaAudio
	default:
		return fmt.Errorf("invalid presence state: %s", state)
	}

	return c.client.SendChatPresence(ctx, jid, presenceState, presenceMedia)
}

// SubscribeChatPresence subscribes to presence updates for a chat.
func (c *ChatService) SubscribeChatPresence(ctx context.Context, chatID string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}
	return c.client.SubscribePresence(ctx, jid)
}

// SetEphemeralExpiration sets the disappearing messages timer for a chat.
func (c *ChatService) SetEphemeralExpiration(ctx context.Context, chatID string, expiration string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	var duration time.Duration
	switch expiration {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "90d":
		duration = 90 * 24 * time.Hour
	case "off":
		duration = 0
	default:
		return fmt.Errorf("invalid expiration: %s", expiration)
	}

	return c.client.SetDisappearingTimer(ctx, jid, duration, time.Now())
}

// MuteChat mutes or unmutes a chat.
func (c *ChatService) MuteChat(ctx context.Context, chatID string, duration string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	var dur time.Duration
	var muted bool
	switch duration {
	case "8h":
		dur = 8 * time.Hour
		muted = true
	case "1w":
		dur = 7 * 24 * time.Hour
		muted = true
	case "always":
		dur = time.Duration(1<<63 - 1)
		muted = true
	case "off":
		dur = 0
		muted = false
	default:
		return fmt.Errorf("invalid duration: %s (valid: 8h, 1w, always, off)", duration)
	}

	patch := appstate.BuildMute(jid, muted, dur)
	return c.client.SendAppState(context.Background(), patch)
}

// PinChat pins or unpins a chat.
func (c *ChatService) PinChat(ctx context.Context, chatID string, pinned bool) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}
	return c.client.SendAppState(context.Background(), appstate.BuildPin(jid, pinned))
}

// ArchiveChat archives or unarchives a chat.
func (c *ChatService) ArchiveChat(ctx context.Context, chatID string, archived bool) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}
	return c.client.SendAppState(context.Background(), appstate.BuildArchive(jid, archived, time.Now(), nil))
}

// MarkChatAsRead marks a chat as read or unread.
func (c *ChatService) MarkChatAsRead(ctx context.Context, chatID string, read bool) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	patch := buildMarkChatAsRead(jid, read, time.Now())
	return c.client.SendAppState(context.Background(), patch)
}

// ClearChat clears all messages from a chat but keeps the chat in the list.
func (c *ChatService) ClearChat(ctx context.Context, chatID string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	patch := buildClearChat(jid, time.Now())
	return c.client.SendAppState(context.Background(), patch)
}

// DeleteChat deletes a chat entirely.
func (c *ChatService) DeleteChat(ctx context.Context, chatID string) error {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	patch := buildDeleteChat(jid, time.Now())
	return c.client.SendAppState(context.Background(), patch)
}

// GetBusinessProfile retrieves the business profile for a contact.
func (c *ChatService) GetBusinessProfile(ctx context.Context, chatID string) (BusinessProfileResponse, error) {
	jid, err := waTypes.ParseJID(chatID)
	if err != nil {
		return BusinessProfileResponse{}, fmt.Errorf("invalid JID: %w", err)
	}

	profile, err := getBusinessProfileIQ(c.client, jid)
	if err != nil {
		return BusinessProfileResponse{}, fmt.Errorf("failed to get business profile: %w", err)
	}

	return profile, nil
}

// RequestMessages sends an on-demand history sync request for the given chat.
// The response arrives asynchronously as a message_history_sync event.
func (c *ChatService) RequestMessages(ctx context.Context, chatID, lastMessageID, lastMessageSenderID string, count int) error {
	chatJID, err := waTypes.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat JID: %w", err)
	}

	senderJID := FormatRecipient(lastMessageSenderID)
	isGroup := chatJID.Server == waTypes.GroupServer
	isFromMe := senderJID.User == c.client.Store.ID.User

	msgInfo := &waTypes.MessageInfo{
		MessageSource: waTypes.MessageSource{
			Chat:     chatJID,
			Sender:   senderJID,
			IsFromMe: isFromMe,
			IsGroup:  isGroup,
		},
		ID: lastMessageID,
	}

	msg := c.client.BuildHistorySyncRequest(msgInfo, count)
	_, err = c.client.SendPeerMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to request messages: %w", err)
	}
	return nil
}

// buildMarkChatAsRead builds an app state patch for marking a chat as read/unread.
func buildMarkChatAsRead(target waTypes.JID, read bool, ts time.Time) appstate.PatchInfo {
	if ts.IsZero() {
		ts = time.Now()
	}
	return appstate.PatchInfo{
		Type: appstate.WAPatchRegularLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexMarkChatAsRead, target.String()},
			Version: 3,
			Value: &waSyncAction.SyncActionValue{
				MarkChatAsReadAction: &waSyncAction.MarkChatAsReadAction{
					Read: proto.Bool(read),
					MessageRange: &waSyncAction.SyncActionMessageRange{
						LastMessageTimestamp: proto.Int64(ts.Unix()),
					},
				},
			},
		}},
	}
}

// buildDeleteChat builds an app state patch for deleting a chat.
func buildDeleteChat(target waTypes.JID, ts time.Time) appstate.PatchInfo {
	if ts.IsZero() {
		ts = time.Now()
	}
	return appstate.PatchInfo{
		Type: appstate.WAPatchRegularLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexDeleteChat, target.String()},
			Version: 3,
			Value: &waSyncAction.SyncActionValue{
				DeleteChatAction: &waSyncAction.DeleteChatAction{
					MessageRange: &waSyncAction.SyncActionMessageRange{
						LastMessageTimestamp: proto.Int64(ts.Unix()),
					},
				},
			},
		}},
	}
}

// buildClearChat builds an app state patch for clearing a chat's messages.
func buildClearChat(target waTypes.JID, ts time.Time) appstate.PatchInfo {
	if ts.IsZero() {
		ts = time.Now()
	}
	return appstate.PatchInfo{
		Type: appstate.WAPatchRegularLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexClearChat, target.String(), "1", "1"},
			Version: 3,
			Value: &waSyncAction.SyncActionValue{
				ClearChatAction: &waSyncAction.ClearChatAction{
					MessageRange: &waSyncAction.SyncActionMessageRange{
						LastMessageTimestamp: proto.Int64(ts.Unix()),
					},
				},
			},
		}},
	}
}

// getStringContent extracts a string from a waBinary.Node's content.
func getStringContent(node waBinary.Node) string {
	if node.Content == nil {
		return ""
	}
	switch content := node.Content.(type) {
	case string:
		return content
	case []byte:
		return string(content)
	default:
		return ""
	}
}

// getBusinessProfileIQ retrieves a business profile via a raw IQ stanza.
func getBusinessProfileIQ(client *whatsmeow.Client, jid waTypes.JID) (BusinessProfileResponse, error) {
	request := []waBinary.Node{
		{
			Tag: "business_profile",
			Attrs: waBinary.Attrs{
				"v": "244",
			},
			Content: []waBinary.Node{
				{
					Tag: "profile",
					Attrs: waBinary.Attrs{
						"jid": jid,
					},
				},
			},
		},
	}

	resp, err := client.DangerousInternals().SendIQ(context.Background(), whatsmeow.DangerousInfoQuery{
		Type:      "get",
		To:        waTypes.ServerJID,
		Namespace: "w:biz",
		Content:   request,
	})
	if err != nil {
		return BusinessProfileResponse{}, fmt.Errorf("failed to send business profile request: %w", err)
	}

	businessNode, ok := resp.GetOptionalChildByTag("business_profile")
	if !ok {
		return BusinessProfileResponse{}, fmt.Errorf("no business_profile node in response")
	}

	profileNode := businessNode.GetChildByTag("profile")
	profileJID, ok := profileNode.AttrGetter().GetJID("jid", true)
	if !ok {
		return BusinessProfileResponse{}, fmt.Errorf("missing jid in business profile")
	}

	parseFloat := func(s string) float64 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	// Parse business hours
	businessHourNode := profileNode.GetChildByTag("business_hours")
	businessHourTimezone := businessHourNode.AttrGetter().String("timezone")
	var businessHours []BusinessHours
	for _, config := range businessHourNode.GetChildren() {
		if config.Tag != "business_hours_config" {
			continue
		}
		ag := config.AttrGetter()
		businessHours = append(businessHours, BusinessHours{
			DayOfWeek: ag.String("day_of_week"),
			Mode:      ag.String("mode"),
			OpenTime:  ag.String("open_time"),
			CloseTime: ag.String("close_time"),
		})
	}

	// Parse categories
	var categories []BusinessCategory
	categoriesNode := profileNode.GetChildByTag("categories")
	for _, cat := range categoriesNode.GetChildren() {
		if cat.Tag != "category" {
			continue
		}
		categories = append(categories, BusinessCategory{
			ID:   cat.AttrGetter().String("id"),
			Name: getStringContent(cat),
		})
	}

	// Parse profile options
	profileOptions := make(map[string]string)
	profileOptionsNode := profileNode.GetChildByTag("profile_options")
	for _, option := range profileOptionsNode.GetChildren() {
		profileOptions[option.Tag] = getStringContent(option)
	}

	lat := getStringContent(profileNode.GetChildByTag("latitude"))
	lng := getStringContent(profileNode.GetChildByTag("longitude"))

	return BusinessProfileResponse{
		ID:                    profileJID.User,
		Address:               getStringContent(profileNode.GetChildByTag("address")),
		Email:                 getStringContent(profileNode.GetChildByTag("email")),
		Description:           getStringContent(profileNode.GetChildByTag("description")),
		Website:               getStringContent(profileNode.GetChildByTag("website")),
		Latitude:              parseFloat(lat),
		Longitude:             parseFloat(lng),
		MemberSince:           getStringContent(profileNode.GetChildByTag("member_since_text")),
		Categories:            categories,
		BusinessHours:         businessHours,
		BusinessHoursTimeZone: businessHourTimezone,
		ProfileOptions:        profileOptions,
	}, nil
}
