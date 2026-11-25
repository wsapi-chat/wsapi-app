package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/wsapi-chat/wsapi-app/internal/httputil"
)

const maxDownloadSize int64 = 50 * 1024 * 1024 // 50 MB

// allowPrivateMediaURLs lets media URLs reach loopback/private/link-local
// addresses. Off by default so API callers can't use media sends to reach
// services on the server's own network.
var allowPrivateMediaURLs atomic.Bool

// SetAllowPrivateMediaURLs sets whether media URL downloads may reach
// non-public addresses (WSAPI_MEDIA_ALLOW_PRIVATE_URLS).
func SetAllowPrivateMediaURLs(allow bool) {
	allowPrivateMediaURLs.Store(allow)
}

// DownloadMediaFromURL downloads media from a URL with size limits and
// browser-like headers to avoid 406 responses from some servers.
func DownloadMediaFromURL(ctx context.Context, rawURL string) ([]byte, error) {
	return downloadMediaFromURL(ctx, rawURL, httputil.NewPublicClient(30*time.Second, allowPrivateMediaURLs.Load()))
}

func downloadMediaFromURL(ctx context.Context, rawURL string, client *http.Client) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, httputil.ErrUnsupportedScheme
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		// Bare sentinels so the caller gets a clean 400 without dial details.
		if errors.Is(err, httputil.ErrPrivateAddress) {
			return nil, httputil.ErrPrivateAddress
		}
		if errors.Is(err, httputil.ErrUnsupportedScheme) {
			return nil, httputil.ErrUnsupportedScheme
		}
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
