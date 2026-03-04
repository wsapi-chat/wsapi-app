package event

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
)

// Event is the top-level webhook envelope.
type Event struct {
	EventID    string    `json:"eventId"`
	InstanceID string    `json:"instanceId"`
	EventType  string    `json:"eventType"`
	ReceivedAt time.Time `json:"receivedAt"`
	Data       any       `json:"eventData"`
}

// NewEvent constructs an Event with a generated ID and current timestamp.
func NewEvent(instanceID, eventType string, data any) Event {
	return Event{
		EventID:    GenerateID(),
		InstanceID: instanceID,
		EventType:  eventType,
		ReceivedAt: time.Now(),
		Data:       data,
	}
}

func GenerateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("evt_%d_%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// MessageEvent is the typed payload for message events.
type MessageEvent struct {
	ID                  string          `json:"id"`
	ChatID              string          `json:"chatId"`
	Sender              identity.Sender `json:"sender"`
	IsGroup             bool            `json:"isGroup"`
	Time                time.Time       `json:"time"`
	Type                string          `json:"type"`
	Text                string          `json:"text,omitempty"`
	Media               *MediaInfo      `json:"media,omitempty"`
	Reaction            *ReactionInfo   `json:"reaction,omitempty"`
	ReplyTo             *ReplyInfo      `json:"replyTo,omitempty"`
	Contact             string          `json:"contact,omitempty"`
	ContactArray        []string        `json:"contactArray,omitempty"`
	ExtendedText        *ExtendedText   `json:"extendedText,omitempty"`
	Pin                 *PinInfo        `json:"pin,omitempty"`
	Edit                *EditInfo       `json:"edit,omitempty"`
	Mentions            []string        `json:"mentions,omitempty"`
	IsEdit              bool            `json:"isEdit,omitempty"`
	IsForwarded         bool            `json:"isForwarded,omitempty"`
	ViewOnce            bool            `json:"viewOnce,omitempty"`
	IsStatus            bool            `json:"isStatus,omitempty"`
	EphemeralExpiration string          `json:"ephemeralExpiration,omitempty"`
}

type MediaInfo struct {
	ID            string `json:"id,omitempty"`
	MediaType     string `json:"mediaType"`
	MimeType      string `json:"mimeType,omitempty"`
	FileLength    uint64 `json:"fileLength,omitempty"`
	Caption       string `json:"caption,omitempty"`
	Height        uint32 `json:"height,omitempty"`
	Width         uint32 `json:"width,omitempty"`
	Duration      uint32 `json:"duration,omitempty"`
	Filename      string `json:"filename,omitempty"`
	Title         string `json:"title,omitempty"`
	PageCount     uint32 `json:"pageCount,omitempty"`
	JPEGThumbnail []byte `json:"jpegThumbnail,omitempty"`
}

type ReactionInfo struct {
	MessageID string `json:"messageId"`
	Emoji     string `json:"emoji"`
}

type ReplyInfo struct {
	ID          string           `json:"id,omitempty"`
	Sender      *identity.Sender `json:"sender,omitempty"`
	Text        string           `json:"text,omitempty"`
	IsForwarded bool             `json:"isForwarded,omitempty"`
}

type ExtendedText struct {
	MatchedText   string `json:"matchedText,omitempty"`
	Description   string `json:"description,omitempty"`
	Title         string `json:"title,omitempty"`
	JPEGThumbnail []byte `json:"jpegThumbnail,omitempty"`
}

type PinInfo struct {
	Pinned     bool   `json:"pinned"`
	MessageID  string `json:"messageId"`
	Expiration string `json:"expiration"`
}

type EditInfo struct {
	OriginalMessageTime string `json:"originalMessageTime,omitempty"`
	OriginalMessageID   string `json:"originalMessageId,omitempty"`
}

// ReceiptEvent is the typed payload for message read/delivery receipts.
type ReceiptEvent struct {
	ChatID        string           `json:"chatId"`
	Sender        identity.Sender  `json:"sender"`
	MessageSender *identity.Sender `json:"messageSender,omitempty"`
	IsGroup       bool             `json:"isGroup"`
	MessageIDs    []string         `json:"messageIds,omitempty"`
	Time          time.Time        `json:"time"`
	ReceiptType   string           `json:"receiptType"`
}

// MessageDeleteEvent for delete events.
type MessageDeleteEvent struct {
	ID              string          `json:"id"`
	ChatID          string          `json:"chatId"`
	Sender          identity.Sender `json:"sender"`
	Time            time.Time       `json:"time"`
	IsDeletedForAll bool            `json:"isDeletedForAll,omitempty"`
	IsDeletedForMe  bool            `json:"isDeletedForMe,omitempty"`
	IsStatus        bool            `json:"isStatus,omitempty"`
}

// MessageStarEvent for star/unstar events.
type MessageStarEvent struct {
	ID        string          `json:"id"`
	ChatID    string          `json:"chatId"`
	Sender    identity.Sender `json:"sender"`
	Time      time.Time       `json:"time"`
	IsStarred bool            `json:"isStarred"`
}

// ChatSettingEvent for chat setting changes.
type ChatSettingEvent struct {
	ID          string `json:"id"`
	SettingType string `json:"settingType"`
	// Only one of these will be set
	Mute      *ChatMuteSetting      `json:"mute,omitempty"`
	Pin       *ChatPinSetting       `json:"pin,omitempty"`
	Archive   *ChatArchiveSetting   `json:"archive,omitempty"`
	Read      *ChatReadSetting      `json:"read,omitempty"`
	Ephemeral *ChatEphemeralSetting `json:"ephemeral,omitempty"`
}

type ChatMuteSetting struct {
	IsMuted     bool   `json:"isMuted"`
	MuteEndTime string `json:"muteEndTime,omitempty"`
}

type ChatPinSetting struct {
	IsPinned bool `json:"isPinned"`
}

type ChatArchiveSetting struct {
	IsArchived bool `json:"isArchived"`
}

type ChatReadSetting struct {
	IsRead bool `json:"isRead"`
}

type ChatEphemeralSetting struct {
	Expiration string          `json:"expiration"`
	Sender     identity.Sender `json:"sender"`
}

// ChatPresenceEvent for typing/recording indicators.
type ChatPresenceEvent struct {
	ID     string          `json:"id"`
	Sender identity.Sender `json:"sender"`
	State  string          `json:"state"`
}

// ChatPushNameEvent for push name changes.
type ChatPushNameEvent struct {
	User     identity.Identity `json:"user"`
	PushName string            `json:"pushName"`
}

// ChatStatusEvent for about/status text changes.
type ChatStatusEvent struct {
	User   identity.Identity `json:"user"`
	Status string            `json:"status"`
}

// ChatPictureEvent for profile picture changes.
type ChatPictureEvent struct {
	ID        string          `json:"id"`
	Sender    identity.Sender `json:"sender"`
	PictureID string          `json:"pictureId"`
}

// GroupEvent for group-related changes.
type GroupEvent struct {
	ID                  string                       `json:"id"`
	Sender              identity.Sender              `json:"sender"`
	Timestamp           time.Time                    `json:"timestamp,omitempty"`
	Description         *GroupDescriptionInfo        `json:"description,omitempty"`
	Name                *GroupNameInfo               `json:"name,omitempty"`
	Locked              *GroupLockedInfo             `json:"locked,omitempty"`
	Announce            *GroupAnnounceInfo           `json:"announce,omitempty"`
	MembershipApproval  *GroupMembershipApprovalInfo `json:"membershipApproval,omitempty"`
	Delete              *GroupDeleteInfo             `json:"delete,omitempty"`
	Suspended           *bool                        `json:"suspended,omitempty"`
	Leave               []identity.Identity          `json:"leave,omitempty"`
	Join                []identity.Identity          `json:"join,omitempty"`
	Promote             []identity.Identity          `json:"promote,omitempty"`
	Demote              []identity.Identity          `json:"demote,omitempty"`
	Link                *GroupLinkInfo               `json:"link,omitempty"`
	Unlink              *GroupUnlinkInfo             `json:"unlink,omitempty"`
	JoinReason          string                       `json:"joinReason,omitempty"`
	GroupType           string                       `json:"groupType,omitempty"`
	IsCommunity         bool                         `json:"isCommunity,omitempty"`
	CommunityID         string                       `json:"communityId,omitempty"`
	IsAnnouncementGroup bool                         `json:"isAnnouncementGroup,omitempty"`
}

type GroupDescriptionInfo struct {
	Topic string `json:"topic"`
}

type GroupNameInfo struct {
	Name string `json:"name"`
}

type GroupLockedInfo struct {
	IsLocked bool `json:"isLocked"`
}

type GroupAnnounceInfo struct {
	IsAnnounce bool `json:"isAnnounce"`
}

type GroupLinkInfo struct {
	Type                string `json:"type"`
	GroupID             string `json:"groupId"`
	GroupName           string `json:"groupName"`
	IsAnnouncementGroup bool   `json:"isAnnouncementGroup"`
}

type GroupUnlinkInfo struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	GroupID string `json:"groupId"`
}

type GroupMembershipApprovalInfo struct {
	IsRequired bool `json:"isRequired"`
}

type GroupDeleteInfo struct {
	Reason string `json:"reason,omitempty"`
}

// NewsletterEvent for newsletter subscription and mute changes.
type NewsletterEvent struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
	Role   string `json:"role,omitempty"`
	Mute   string `json:"mute,omitempty"`
}

// ContactEvent for contact changes.
type ContactEvent struct {
	Contact            identity.Identity `json:"contact"`
	FullName           string            `json:"fullName"`
	InPhoneAddressBook bool              `json:"inPhoneAddressBook"`
}

// PresenceEvent for user presence.
type PresenceEvent struct {
	User     identity.Identity `json:"user"`
	Status   string            `json:"status"`
	LastSeen time.Time         `json:"lastSeen,omitempty"`
}

// CallOfferEvent for incoming calls.
type CallOfferEvent struct {
	ID      string            `json:"id"`
	Caller  identity.Identity `json:"caller"`
	ChatID  string            `json:"chatId"`
	IsGroup bool              `json:"isGroup"`
	Time    time.Time         `json:"time"`
	IsVideo bool              `json:"isVideo"`
}

// CallTerminateEvent for call terminations.
type CallTerminateEvent struct {
	ID     string            `json:"id"`
	Caller identity.Identity `json:"caller"`
	Time   time.Time         `json:"time"`
	Reason string            `json:"reason"`
}

// CallAcceptEvent for accepted calls.
type CallAcceptEvent struct {
	ID     string            `json:"id"`
	Caller identity.Identity `json:"caller"`
	Time   time.Time         `json:"time"`
}

// SessionLoggedInEvent for login events.
type SessionLoggedInEvent struct {
	DeviceID string `json:"deviceId"`
}

// SessionLoggedOutEvent for logout events.
type SessionLoggedOutEvent struct {
	Reason string `json:"reason"`
}

// SessionLoginErrorEvent for login error events.
type SessionLoginErrorEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// HistorySyncEvent for history sync messages.
type HistorySyncEvent struct {
	ChatID   string         `json:"chatId,omitempty"`
	Messages []MessageEvent `json:"messages,omitempty"`
}

// InitialSyncFinishedEvent signals that the initial history sync has completed.
type InitialSyncFinishedEvent struct{}
