package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waTypes "go.mau.fi/whatsmeow/types"
)

// PrivacySettingsResponse is the domain response type for privacy settings.
type PrivacySettingsResponse struct {
	GroupAdd     string `json:"groupAdd"`
	LastSeen     string `json:"lastSeen"`
	Status       string `json:"status"`
	Profile      string `json:"profile"`
	ReadReceipts string `json:"readReceipts"`
	Online       string `json:"online"`
	CallAdd      string `json:"callAdd"`
}

// UserMeInfo is the domain response type for the logged-in user's profile.
type UserMeInfo struct {
	identity.Identity
	DeviceID     uint16 `json:"deviceId"`
	PushName     string `json:"pushName"`
	BusinessName string `json:"businessName"`
	Status       string `json:"status"`
	PictureID    string `json:"pictureId"`
	IsVerified   bool   `json:"isVerified"`
}

// UserMeService wraps the whatsmeow client for own-account operations.
type UserMeService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// GetMyInfo returns information about the currently logged-in account.
func (a *UserMeService) GetMyInfo(ctx context.Context) (UserMeInfo, error) {
	if a.client.Store.ID == nil {
		return UserMeInfo{}, fmt.Errorf("no user found")
	}

	jidWithDevice := *a.client.Store.ID
	jid, _ := waTypes.ParseJID(jidWithDevice.User + "@" + jidWithDevice.Server)

	usersInfo, err := a.client.GetUserInfo(ctx, []waTypes.JID{jid})
	if err != nil {
		return UserMeInfo{}, fmt.Errorf("failed to get user info: %v", err)
	}

	// Use Device.LID for the alt JID.
	lidJID := a.client.Store.GetLID()
	userDetails := usersInfo[jid]

	return UserMeInfo{
		Identity:     identity.Resolve(ctx, jid, lidJID, a.client.Store.LIDs),
		DeviceID:     jidWithDevice.Device,
		PushName:     a.client.Store.PushName,
		BusinessName: a.client.Store.BusinessName,
		Status:       userDetails.Status,
		PictureID:    userDetails.PictureID,
		IsVerified:   userDetails.VerifiedName != nil,
	}, nil
}

// SetName sets the push name for the WhatsApp account.
func (a *UserMeService) SetName(ctx context.Context, name string) error {
	err := a.client.SendAppState(ctx, appstate.BuildSettingPushName(name))
	if err != nil {
		return fmt.Errorf("failed to set push name: %v", err)
	}
	return nil
}

// SetProfilePicture sets the profile picture for the WhatsApp account.
func (a *UserMeService) SetProfilePicture(ctx context.Context, picture []byte) (string, error) {
	emptyJID := waTypes.JID{}
	return a.client.SetGroupPhoto(ctx, emptyJID, picture)
}

// SetStatus sets the about/status text for the WhatsApp account.
func (a *UserMeService) SetStatus(ctx context.Context, status string) error {
	return a.client.SetStatusMessage(ctx, status)
}

// SendPresence sets the presence state (available or unavailable).
func (a *UserMeService) SendPresence(ctx context.Context, presenceType string) error {
	var state waTypes.Presence
	switch strings.ToLower(presenceType) {
	case "available":
		state = waTypes.PresenceAvailable
	case "unavailable":
		state = waTypes.PresenceUnavailable
	default:
		return fmt.Errorf("unsupported presence type: %s", presenceType)
	}
	return a.client.SendPresence(ctx, state)
}

// GetPrivacySettings returns the current privacy settings.
func (a *UserMeService) GetPrivacySettings(ctx context.Context) (PrivacySettingsResponse, error) {
	settings, err := a.client.TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return PrivacySettingsResponse{}, fmt.Errorf("failed to get privacy settings: %w", err)
	}
	return PrivacySettingsResponse{
		GroupAdd:     string(settings.GroupAdd),
		LastSeen:     string(settings.LastSeen),
		Status:       string(settings.Status),
		Profile:      string(settings.Profile),
		ReadReceipts: string(settings.ReadReceipts),
		Online:       string(settings.Online),
		CallAdd:      string(settings.CallAdd),
	}, nil
}

// SetPrivacySetting updates a single privacy setting.
func (a *UserMeService) SetPrivacySetting(ctx context.Context, settingType, value string) (PrivacySettingsResponse, error) {
	updated, err := a.client.SetPrivacySetting(ctx, waTypes.PrivacySettingType(settingType), waTypes.PrivacySetting(value))
	if err != nil {
		return PrivacySettingsResponse{}, fmt.Errorf("failed to set privacy setting: %w", err)
	}
	return PrivacySettingsResponse{
		GroupAdd:     string(updated.GroupAdd),
		LastSeen:     string(updated.LastSeen),
		Status:       string(updated.Status),
		Profile:      string(updated.Profile),
		ReadReceipts: string(updated.ReadReceipts),
		Online:       string(updated.Online),
		CallAdd:      string(updated.CallAdd),
	}, nil
}
