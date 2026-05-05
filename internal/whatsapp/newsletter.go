package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mau.fi/whatsmeow"
	waTypes "go.mau.fi/whatsmeow/types"
)

// NewsletterInfoResponse is the domain response type for newsletter information.
type NewsletterInfoResponse struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	SubscriberCount   int       `json:"subscriberCount"`
	VerificationState string    `json:"verificationState"`
	PictureURL        string    `json:"pictureUrl,omitempty"`
	InviteCode        string    `json:"inviteCode,omitempty"`
	Role              string    `json:"role,omitempty"`
	Mute              string    `json:"mute,omitempty"`
	State             string    `json:"state"`
	CreatedAt         time.Time `json:"createdAt"`
}

// NewsletterService wraps the whatsmeow client for newsletter operations.
type NewsletterService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// GetSubscribedNewsletters returns all newsletters the user is subscribed to.
func (n *NewsletterService) GetSubscribedNewsletters(ctx context.Context) ([]NewsletterInfoResponse, error) {
	newsletters, err := n.client.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscribed newsletters: %w", err)
	}
	result := make([]NewsletterInfoResponse, 0, len(newsletters))
	for _, nl := range newsletters {
		result = append(result, toNewsletterInfoResponse(nl))
	}
	return result, nil
}

// GetNewsletterInfo returns info about a specific newsletter.
func (n *NewsletterService) GetNewsletterInfo(ctx context.Context, newsletterID string) (NewsletterInfoResponse, error) {
	jid, err := parseJID(newsletterID)
	if err != nil {
		return NewsletterInfoResponse{}, fmt.Errorf("invalid newsletter JID: %w", err)
	}
	info, err := n.client.GetNewsletterInfo(ctx, jid)
	if err != nil {
		return NewsletterInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return toNewsletterInfoResponse(info), nil
}

// GetNewsletterInfoWithInvite returns info about a newsletter using an invite code.
func (n *NewsletterService) GetNewsletterInfoWithInvite(ctx context.Context, inviteCode string) (NewsletterInfoResponse, error) {
	info, err := n.client.GetNewsletterInfoWithInvite(ctx, inviteCode)
	if err != nil {
		return NewsletterInfoResponse{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return toNewsletterInfoResponse(info), nil
}

// CreateNewsletter creates a new newsletter.
func (n *NewsletterService) CreateNewsletter(ctx context.Context, name, description string, picture []byte) (NewsletterInfoResponse, error) {
	if name == "" {
		return NewsletterInfoResponse{}, fmt.Errorf("newsletter name cannot be empty")
	}
	params := whatsmeow.CreateNewsletterParams{
		Name:        name,
		Description: description,
		Picture:     picture,
	}
	info, err := n.client.CreateNewsletter(ctx, params)
	if err != nil {
		return NewsletterInfoResponse{}, fmt.Errorf("failed to create newsletter: %w", err)
	}
	return toNewsletterInfoResponse(info), nil
}

// FollowNewsletter subscribes to a newsletter.
func (n *NewsletterService) FollowNewsletter(ctx context.Context, newsletterID string) error {
	jid, err := parseJID(newsletterID)
	if err != nil {
		return fmt.Errorf("invalid newsletter JID: %w", err)
	}
	return n.client.FollowNewsletter(ctx, jid)
}

// UnfollowNewsletter unsubscribes from a newsletter.
func (n *NewsletterService) UnfollowNewsletter(ctx context.Context, newsletterID string) error {
	jid, err := parseJID(newsletterID)
	if err != nil {
		return fmt.Errorf("invalid newsletter JID: %w", err)
	}
	return n.client.UnfollowNewsletter(ctx, jid)
}

// ToggleMuteNewsletter mutes or unmutes a newsletter.
func (n *NewsletterService) ToggleMuteNewsletter(ctx context.Context, newsletterID string, mute bool) error {
	jid, err := parseJID(newsletterID)
	if err != nil {
		return fmt.Errorf("invalid newsletter JID: %w", err)
	}
	return n.client.NewsletterToggleMute(ctx, jid, mute)
}

// toNewsletterInfoResponse converts a whatsmeow NewsletterMetadata to the domain response.
func toNewsletterInfoResponse(info *waTypes.NewsletterMetadata) NewsletterInfoResponse {
	if info == nil {
		return NewsletterInfoResponse{}
	}
	resp := NewsletterInfoResponse{
		ID:                info.ID.String(),
		Name:              info.ThreadMeta.Name.Text,
		Description:       info.ThreadMeta.Description.Text,
		SubscriberCount:   info.ThreadMeta.SubscriberCount,
		VerificationState: string(info.ThreadMeta.VerificationState),
		InviteCode:        info.ThreadMeta.InviteCode,
		State:             string(info.State.Type),
		CreatedAt:         info.ThreadMeta.CreationTime.Time,
	}
	if info.ThreadMeta.Picture != nil {
		resp.PictureURL = info.ThreadMeta.Picture.URL
	}
	if info.ViewerMeta != nil {
		resp.Role = string(info.ViewerMeta.Role)
		resp.Mute = string(info.ViewerMeta.Mute)
	}
	return resp
}
