package whatsapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/httputil"
)

const maxDownloadSize int64 = 50 * 1024 * 1024 // 50 MB

// DownloadMediaFromURL downloads media from a URL with size limits and
// browser-like headers to avoid 406 responses from some servers.
func DownloadMediaFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := httputil.NewClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.ContentLength > maxDownloadSize {
		return nil, fmt.Errorf("file size %d bytes exceeds limit of %d bytes", resp.ContentLength, maxDownloadSize)
	}

	data, err := io.ReadAll(&io.LimitedReader{R: resp.Body, N: maxDownloadSize + 1})
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(data)) > maxDownloadSize {
		return nil, fmt.Errorf("file exceeds size limit of %d bytes", maxDownloadSize)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download from URL: status code %d", resp.StatusCode)
	}

	return data, nil
}
