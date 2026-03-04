package whatsapp

import (
	"context"
	"fmt"
	"log/slog"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	waTypes "go.mau.fi/whatsmeow/types"

	"google.golang.org/protobuf/proto"
)

// DeleteStatus revokes (deletes) a previously posted status update.
func (s *StatusService) DeleteStatus(ctx context.Context, messageID string) error {
	msg := s.client.BuildRevoke(waTypes.StatusBroadcastJID, waTypes.EmptyJID, messageID)
	_, err := s.client.SendMessage(ctx, waTypes.StatusBroadcastJID, msg)
	if err != nil {
		return fmt.Errorf("failed to delete status: %w", err)
	}
	return nil
}

// StatusService wraps the whatsmeow client for status/stories operations.
type StatusService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// PostTextStatus posts a text status update.
func (s *StatusService) PostTextStatus(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("status text cannot be empty")
	}
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	}
	resp, err := s.client.SendMessage(ctx, waTypes.StatusBroadcastJID, msg)
	if err != nil {
		return "", fmt.Errorf("failed to post text status: %w", err)
	}
	return resp.ID, nil
}

// PostImageStatus posts an image status update.
func (s *StatusService) PostImageStatus(ctx context.Context, data []byte, mimeType, caption string) (string, error) {
	uploaded, err := s.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}
	resp, err := s.client.SendMessage(ctx, waTypes.StatusBroadcastJID, msg)
	if err != nil {
		return "", fmt.Errorf("failed to post image status: %w", err)
	}
	return resp.ID, nil
}

// StatusPrivacyResponse is the domain response type for a status privacy setting.
type StatusPrivacyResponse struct {
	Type      string   `json:"type"`
	List      []string `json:"list,omitempty"`
	IsDefault bool     `json:"isDefault"`
}

// GetStatusPrivacy returns the status broadcast privacy settings.
func (s *StatusService) GetStatusPrivacy(ctx context.Context) ([]StatusPrivacyResponse, error) {
	privacy, err := s.client.GetStatusPrivacy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status privacy: %w", err)
	}
	result := make([]StatusPrivacyResponse, 0, len(privacy))
	for _, p := range privacy {
		jids := make([]string, 0, len(p.List))
		for _, jid := range p.List {
			jids = append(jids, CleanJID(jid).String())
		}
		result = append(result, StatusPrivacyResponse{
			Type:      string(p.Type),
			List:      jids,
			IsDefault: p.IsDefault,
		})
	}
	return result, nil
}

// PostVideoStatus posts a video status update.
func (s *StatusService) PostVideoStatus(ctx context.Context, data []byte, mimeType, caption string) (string, error) {
	uploaded, err := s.client.Upload(ctx, data, whatsmeow.MediaVideo)
	if err != nil {
		return "", fmt.Errorf("failed to upload video: %w", err)
	}
	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}
	resp, err := s.client.SendMessage(ctx, waTypes.StatusBroadcastJID, msg)
	if err != nil {
		return "", fmt.Errorf("failed to post video status: %w", err)
	}
	return resp.ID, nil
}
