package httputil

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func TestIsPublicAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.8.8.8", "::1",
		"10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"fc00::1", "fd12:3456::1",
		"169.254.169.254", "169.254.0.1", "fe80::1",
		"0.0.0.0", "0.1.2.3", "::",
		"100.64.0.1", "100.127.255.255",
		"224.0.0.1", "239.255.255.250", "255.255.255.255", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254",
	}
	for _, s := range blocked {
		if IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("IsPublicAddr(%s) = true, want false", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "100.63.255.255", "100.128.0.1", "172.32.0.1", "2001:4860:4860::8888"}
	for _, s := range allowed {
		if !IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("IsPublicAddr(%s) = false, want true", s)
		}
	}
}

func TestPublicClientBlocksInternalTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("secret"))
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	client := NewPublicClient(5*time.Second, false)
	targets := []string{
		srv.URL,                    // 127.0.0.1
		"http://localhost:" + port, // hostname resolving to loopback
		"http://10.255.255.1/",     // RFC1918, rejected before connecting
		"http://169.254.169.254/latest/meta-data", // link-local
		"http://[::1]:" + port,
	}
	for _, target := range targets {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("GET %s succeeded, want ErrPrivateAddress", target)
			continue
		}
		if !errors.Is(err, ErrPrivateAddress) {
			t.Errorf("GET %s: err = %v, want ErrPrivateAddress", target, err)
		}
	}
}

func TestPublicClientAllowPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	resp, err := NewPublicClient(5*time.Second, true).Get(srv.URL)
	if err != nil {
		t.Fatalf("GET with allowPrivate: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// A public server that redirects to an internal one must be stopped at the
// redirect: the dial check runs on every hop.
func TestPublicClientBlocksRedirectToInternal(t *testing.T) {
	internalHit := false
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHit = true
		_, _ = w.Write([]byte("secret"))
	}))
	defer internal.Close()

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL+"/private", http.StatusFound)
	}))
	defer public.Close()

	// httptest only listens on loopback: treat the "public" server as public
	// and run the real check for everything else.
	publicAP := netip.MustParseAddrPort(public.Listener.Addr().String())
	check := func(ap netip.AddrPort) error {
		if ap == publicAP {
			return nil
		}
		return requirePublic(ap)
	}

	resp, err := newPublicClient(5*time.Second, false, check).Get(public.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("redirect to loopback succeeded, want ErrPrivateAddress")
	}
	if !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("err = %v, want ErrPrivateAddress", err)
	}
	if internalHit {
		t.Fatal("internal server was reached")
	}
}

// With a proxy the dial goes to the proxy, so the target is resolved and
// checked before each request instead.
func TestPublicClientViaProxyChecksTarget(t *testing.T) {
	proxyHit := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHit = true
		_, _ = w.Write([]byte("proxied"))
	}))
	defer proxy.Close()

	prev := proxyURL
	proxyURL, _ = url.Parse(proxy.URL)
	defer func() { proxyURL = prev }()

	for _, target := range []string{"http://127.0.0.1:9/", "http://localhost/", "http://169.254.169.254/"} {
		resp, err := NewPublicClient(5*time.Second, false).Get(target)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("GET %s via proxy succeeded, want ErrPrivateAddress", target)
			continue
		}
		if !errors.Is(err, ErrPrivateAddress) {
			t.Errorf("GET %s via proxy: err = %v, want ErrPrivateAddress", target, err)
		}
	}
	if proxyHit {
		t.Fatal("proxy was asked for an internal target")
	}

	// The proxy itself lives on loopback here and must still be usable.
	resp, err := NewPublicClient(5*time.Second, true).Get("http://10.0.0.1/")
	if err != nil {
		t.Fatalf("GET via proxy with allowPrivate: %v", err)
	}
	_ = resp.Body.Close()
	if !proxyHit {
		t.Fatal("request did not go through the proxy")
	}
}
