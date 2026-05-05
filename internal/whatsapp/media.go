package whatsapp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// mediaDownloader is an internal interface for downloading media.
// *whatsmeow.Client satisfies this interface.
type mediaDownloader interface {
	Download(ctx context.Context, msg whatsmeow.DownloadableMessage) ([]byte, error)
}

// MediaService wraps the whatsmeow client for media download operations.
type MediaService struct {
	dl          mediaDownloader
	logger      *slog.Logger
	maxFileSize int64
}

// MediaDownloadResult holds the result of a media download.
// The caller must call Close when done reading.
type MediaDownloadResult struct {
	// Body streams the decrypted media content from a temp file.
	Body io.ReadCloser
	// Size is the number of decrypted bytes.
	Size int64
	// Filename is the suggested filename.
	Filename string
	// MimeType is the media MIME type.
	MimeType string
}

// DownloadByID downloads and decrypts a media file using an encoded media ID.
// The returned result streams content from a temp file to keep memory bounded.
func (m *MediaService) DownloadByID(ctx context.Context, mediaID string) (*MediaDownloadResult, error) {
	mediaInfo, err := event.DecodeMediaID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode media ID: %v", err)
	}

	if m.maxFileSize > 0 && mediaInfo.FileLength > uint64(m.maxFileSize) {
		return nil, fmt.Errorf("%w: %d bytes exceeds limit of %d bytes",
			ErrTooLarge, mediaInfo.FileLength, m.maxFileSize)
	}

	downloadableMsg := buildDownloadableMessage(mediaInfo)
	if downloadableMsg == nil {
		return nil, fmt.Errorf("unsupported media type: %s", mediaInfo.MediaType)
	}

	data, err := m.dl.Download(ctx, downloadableMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %v", err)
	}

	// Post-download size check: the pre-download FileLength comes from the
	// client-provided media ID and could be spoofed. Verify against the
	// actual downloaded size to prevent OOM from crafted media IDs.
	if m.maxFileSize > 0 && int64(len(data)) > m.maxFileSize {
		return nil, fmt.Errorf("%w: downloaded %d bytes exceeds limit of %d bytes",
			ErrTooLarge, len(data), m.maxFileSize)
	}

	// Spill to temp file so the []byte can be GC'd before streaming the response.
	f, err := os.CreateTemp("", "wsapi-media-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}

	// Unlink immediately — the fd keeps the file alive until Close.
	_ = os.Remove(f.Name())

	size := int64(len(data))
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to write temp file: %v", err)
	}

	// Release the large buffer so GC can reclaim it.
	data = nil //nolint:ineffassign

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to seek temp file: %v", err)
	}

	return &MediaDownloadResult{
		Body:     f,
		Size:     size,
		Filename: generateFilename(mediaInfo.MediaType, mediaInfo.MimeType, mediaInfo.FileName),
		MimeType: mediaInfo.MimeType,
	}, nil
}

func buildDownloadableMessage(info event.MediaDownloadInfo) whatsmeow.DownloadableMessage {
	switch info.MediaType {
	case "image":
		return &waE2E.ImageMessage{
			URL:           proto.String(info.URL),
			DirectPath:    proto.String(info.DirectPath),
			MediaKey:      info.MediaKey,
			Mimetype:      proto.String(info.MimeType),
			FileLength:    proto.Uint64(info.FileLength),
			FileSHA256:    info.FileSHA256,
			FileEncSHA256: info.FileEncSHA256,
		}
	case "video":
		return &waE2E.VideoMessage{
			URL:           proto.String(info.URL),
			DirectPath:    proto.String(info.DirectPath),
			MediaKey:      info.MediaKey,
			Mimetype:      proto.String(info.MimeType),
			FileLength:    proto.Uint64(info.FileLength),
			FileSHA256:    info.FileSHA256,
			FileEncSHA256: info.FileEncSHA256,
		}
	case "audio", "voice":
		return &waE2E.AudioMessage{
			URL:           proto.String(info.URL),
			DirectPath:    proto.String(info.DirectPath),
			MediaKey:      info.MediaKey,
			Mimetype:      proto.String(info.MimeType),
			FileLength:    proto.Uint64(info.FileLength),
			FileSHA256:    info.FileSHA256,
			FileEncSHA256: info.FileEncSHA256,
			PTT:           proto.Bool(info.MediaType == "voice"),
		}
	case "document":
		return &waE2E.DocumentMessage{
			URL:           proto.String(info.URL),
			DirectPath:    proto.String(info.DirectPath),
			MediaKey:      info.MediaKey,
			Mimetype:      proto.String(info.MimeType),
			FileLength:    proto.Uint64(info.FileLength),
			FileSHA256:    info.FileSHA256,
			FileEncSHA256: info.FileEncSHA256,
			FileName:      proto.String(info.FileName),
		}
	case "sticker":
		return &waE2E.StickerMessage{
			URL:           proto.String(info.URL),
			DirectPath:    proto.String(info.DirectPath),
			MediaKey:      info.MediaKey,
			Mimetype:      proto.String(info.MimeType),
			FileLength:    proto.Uint64(info.FileLength),
			FileSHA256:    info.FileSHA256,
			FileEncSHA256: info.FileEncSHA256,
		}
	default:
		return nil
	}
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
