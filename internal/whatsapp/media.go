package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// MediaService wraps the whatsmeow client for media download operations.
type MediaService struct {
	client *whatsmeow.Client
	logger *slog.Logger
}

// DownloadByID downloads and decrypts a media file using an encoded media ID.
func (m *MediaService) DownloadByID(ctx context.Context, mediaID string) (data []byte, filename, mimeType string, err error) {
	mediaInfo, err := event.DecodeMediaID(mediaID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode media ID: %v", err)
	}

	var downloadableMsg whatsmeow.DownloadableMessage

	switch mediaInfo.MediaType {
	case "image":
		downloadableMsg = &waE2E.ImageMessage{
			URL:           proto.String(mediaInfo.URL),
			DirectPath:    proto.String(mediaInfo.DirectPath),
			MediaKey:      mediaInfo.MediaKey,
			Mimetype:      proto.String(mediaInfo.MimeType),
			FileLength:    proto.Uint64(mediaInfo.FileLength),
			FileSHA256:    mediaInfo.FileSHA256,
			FileEncSHA256: mediaInfo.FileEncSHA256,
		}
	case "video":
		downloadableMsg = &waE2E.VideoMessage{
			URL:           proto.String(mediaInfo.URL),
			DirectPath:    proto.String(mediaInfo.DirectPath),
			MediaKey:      mediaInfo.MediaKey,
			Mimetype:      proto.String(mediaInfo.MimeType),
			FileLength:    proto.Uint64(mediaInfo.FileLength),
			FileSHA256:    mediaInfo.FileSHA256,
			FileEncSHA256: mediaInfo.FileEncSHA256,
		}
	case "audio", "voice":
		downloadableMsg = &waE2E.AudioMessage{
			URL:           proto.String(mediaInfo.URL),
			DirectPath:    proto.String(mediaInfo.DirectPath),
			MediaKey:      mediaInfo.MediaKey,
			Mimetype:      proto.String(mediaInfo.MimeType),
			FileLength:    proto.Uint64(mediaInfo.FileLength),
			FileSHA256:    mediaInfo.FileSHA256,
			FileEncSHA256: mediaInfo.FileEncSHA256,
			PTT:           proto.Bool(mediaInfo.MediaType == "voice"),
		}
	case "document":
		downloadableMsg = &waE2E.DocumentMessage{
			URL:           proto.String(mediaInfo.URL),
			DirectPath:    proto.String(mediaInfo.DirectPath),
			MediaKey:      mediaInfo.MediaKey,
			Mimetype:      proto.String(mediaInfo.MimeType),
			FileLength:    proto.Uint64(mediaInfo.FileLength),
			FileSHA256:    mediaInfo.FileSHA256,
			FileEncSHA256: mediaInfo.FileEncSHA256,
			FileName:      proto.String(mediaInfo.FileName),
		}
	case "sticker":
		downloadableMsg = &waE2E.StickerMessage{
			URL:           proto.String(mediaInfo.URL),
			DirectPath:    proto.String(mediaInfo.DirectPath),
			MediaKey:      mediaInfo.MediaKey,
			Mimetype:      proto.String(mediaInfo.MimeType),
			FileLength:    proto.Uint64(mediaInfo.FileLength),
			FileSHA256:    mediaInfo.FileSHA256,
			FileEncSHA256: mediaInfo.FileEncSHA256,
		}
	default:
		return nil, "", "", fmt.Errorf("unsupported media type: %s", mediaInfo.MediaType)
	}

	data, err = m.client.Download(ctx, downloadableMsg)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to download media: %v", err)
	}

	filename = generateFilename(mediaInfo.MediaType, mediaInfo.MimeType, mediaInfo.FileName)

	return data, filename, mediaInfo.MimeType, nil
}

// generateFilename creates an appropriate filename based on media type and metadata.
func generateFilename(mediaType, mimeType, originalFileName string) string {
	if originalFileName != "" {
		return originalFileName
	}

	timestamp := time.Now().Format("20060102_150405")
	baseFilename := fmt.Sprintf("whatsapp_media_%s", timestamp)

	switch mediaType {
	case "image":
		if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
			return baseFilename + ".jpg"
		} else if strings.Contains(mimeType, "png") {
			return baseFilename + ".png"
		} else if strings.Contains(mimeType, "webp") {
			return baseFilename + ".webp"
		}
		return baseFilename + ".jpg"
	case "video":
		if strings.Contains(mimeType, "mp4") {
			return baseFilename + ".mp4"
		} else if strings.Contains(mimeType, "avi") {
			return baseFilename + ".avi"
		} else if strings.Contains(mimeType, "mov") {
			return baseFilename + ".mov"
		}
		return baseFilename + ".mp4"
	case "audio", "voice":
		if strings.Contains(mimeType, "ogg") {
			return baseFilename + ".ogg"
		} else if strings.Contains(mimeType, "mp3") {
			return baseFilename + ".mp3"
		} else if strings.Contains(mimeType, "wav") {
			return baseFilename + ".wav"
		}
		return baseFilename + ".ogg"
	case "document":
		if strings.Contains(mimeType, "pdf") {
			return baseFilename + ".pdf"
		} else if strings.Contains(mimeType, "zip") {
			return baseFilename + ".zip"
		} else if strings.Contains(mimeType, "excel") || strings.Contains(mimeType, "spreadsheet") {
			return baseFilename + ".xlsx"
		} else if strings.Contains(mimeType, "word") || strings.Contains(mimeType, "document") {
			return baseFilename + ".docx"
		}
		return baseFilename + ".bin"
	case "sticker":
		return baseFilename + ".webp"
	default:
		return baseFilename + ".bin"
	}
}
