package whatsapp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wsapi-chat/wsapi-app/internal/httputil"
)

func TestDownloadMediaFromURLRejectsInternal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("private"))
	}))
	defer srv.Close()

	SetAllowPrivateMediaURLs(false)
	_, err := DownloadMediaFromURL(context.Background(), srv.URL+"/private")
	if !errors.Is(err, httputil.ErrPrivateAddress) || err.Error() != httputil.ErrPrivateAddress.Error() {
		t.Fatalf("err = %v, want bare ErrPrivateAddress", err)
	}
}

func TestDownloadMediaFromURLAllowPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("media"))
	}))
	defer srv.Close()

	SetAllowPrivateMediaURLs(true)
	defer SetAllowPrivateMediaURLs(false)
	data, err := DownloadMediaFromURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("download with private URLs allowed: %v", err)
	}
	if string(data) != "media" {
		t.Fatalf("data = %q, want %q", data, "media")
	}
}

func TestDownloadMediaFromURLRejectsScheme(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/a.pdf", "gopher://example.com/"} {
		_, err := DownloadMediaFromURL(context.Background(), u)
		if !errors.Is(err, httputil.ErrUnsupportedScheme) {
			t.Errorf("%s: err = %v, want ErrUnsupportedScheme", u, err)
		}
	}
}
