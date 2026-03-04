package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
	"go.mau.fi/whatsmeow"
	waTypes "go.mau.fi/whatsmeow/types"
)

// UserInfo is the domain response type for user information.
type UserInfo struct {
	identity.Identity
	IsInWhatsApp bool   `json:"isInWhatsApp"`
	Status       string `json:"status"`
	PictureID    string `json:"pictureId"`
	PictureURL   string `json:"pictureUrl,omitempty"`
	IsVerified   bool   `json:"isVerified"`
}

// BulkCheckResult is the domain response type for a bulk WhatsApp check.
type BulkCheckResult struct {
	Query        string `json:"query"`
	IsInWhatsApp bool   `json:"isInWhatsApp"`
	JID          string `json:"jid,omitempty"`
}

// UserService wraps the whatsmeow client for user lookup operations.
type UserService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// GetUserInfo looks up a phone number and returns user information.
func (u *UserService) GetUserInfo(ctx context.Context, phone string) (*UserInfo, error) {
	if !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	isInWhatsApp, err := u.client.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return nil, err
	}

	if len(isInWhatsApp) == 0 {
		return nil, fmt.Errorf("no result for phone number")
	}

	parsedJID := isInWhatsApp[0].JID

	waUserInfo, err := u.client.GetUserInfo(ctx, []waTypes.JID{parsedJID})
	if err != nil || len(waUserInfo) == 0 {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}

	// Use the LID from the user info response as the alt JID.
	userDetails := waUserInfo[parsedJID]
	altJID := userDetails.LID

	info := &UserInfo{
		Identity:     identity.Resolve(ctx, parsedJID, altJID, u.client.Store.LIDs),
		IsInWhatsApp: isInWhatsApp[0].IsIn,
		Status:       userDetails.Status,
		PictureID:    userDetails.PictureID,
		IsVerified:   userDetails.VerifiedName != nil,
	}

	// Fetch profile picture URL.
	if pic, picErr := u.client.GetProfilePictureInfo(ctx, parsedJID, &whatsmeow.GetProfilePictureParams{}); picErr == nil && pic != nil {
		info.PictureURL = pic.URL
	}

	return info, nil
}

// BulkCheckOnWhatsApp checks multiple phone numbers at once.
func (u *UserService) BulkCheckOnWhatsApp(ctx context.Context, phones []string) ([]BulkCheckResult, error) {
	normalized := make([]string, len(phones))
	for i, p := range phones {
		if !strings.HasPrefix(p, "+") {
			p = "+" + p
		}
		normalized[i] = p
	}

	results, err := u.client.IsOnWhatsApp(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("failed to check phone numbers: %w", err)
	}

	out := make([]BulkCheckResult, 0, len(results))
	for _, r := range results {
		item := BulkCheckResult{
			Query:        r.Query,
			IsInWhatsApp: r.IsIn,
		}
		if r.IsIn {
			item.JID = r.JID.String()
		}
		out = append(out, item)
	}
	return out, nil
}

// CheckOnWhatsApp checks whether a phone number is registered on WhatsApp.
func (u *UserService) CheckOnWhatsApp(ctx context.Context, phone string) (bool, error) {
	if !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	result, err := u.client.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return false, err
	}

	if len(result) == 0 {
		return false, nil
	}

	return result[0].IsIn, nil
}
