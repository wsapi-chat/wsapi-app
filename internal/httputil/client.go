package httputil

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var proxyURL *url.URL

// Init parses the proxy URL and stores it for use by NewClient.
// Call this once at startup. If rawURL is empty, no proxy is configured.
func Init(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	proxyURL = u
	return nil
}

// NewClient returns an http.Client with the given timeout. If a proxy was
// configured via Init, the client uses a transport that routes through it.
func NewClient(timeout time.Duration) *http.Client {
	c := &http.Client{Timeout: timeout}
	if proxyURL != nil {
		c.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}
	return c
}
