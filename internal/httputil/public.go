package httputil

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"syscall"
	"time"
)

// ErrPrivateAddress is returned when a request for a user-supplied URL would
// reach a loopback, private, link-local or otherwise internal address.
var ErrPrivateAddress = errors.New("url resolves to a private or internal address")

// ErrUnsupportedScheme is returned for user-supplied URLs that aren't http(s).
var ErrUnsupportedScheme = errors.New("url scheme must be http or https")

var (
	cgnatPrefix       = netip.MustParsePrefix("100.64.0.0/10")
	thisNetworkPrefix = netip.MustParsePrefix("0.0.0.0/8")
	broadcastAddr     = netip.AddrFrom4([4]byte{255, 255, 255, 255})
)

// IsPublicAddr reports whether addr is a globally reachable unicast address.
// IPv4-mapped IPv6 addresses are checked as their IPv4 form.
func IsPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() {
		return false
	}
	if addr.Is4() && (cgnatPrefix.Contains(addr) || thisNetworkPrefix.Contains(addr) || addr == broadcastAddr) {
		return false
	}
	return true
}

// addrCheck decides whether a connection to a resolved address may proceed.
// A parameter so tests can let httptest's loopback server through.
type addrCheck func(netip.AddrPort) error

func requirePublic(ap netip.AddrPort) error {
	if !IsPublicAddr(ap.Addr()) {
		return ErrPrivateAddress
	}
	return nil
}

// NewPublicClient returns a client for fetching user-supplied URLs. Unless
// allowPrivate is set, connections to non-public addresses fail with
// ErrPrivateAddress. The check runs in the dialer on the resolved IP, so it
// also covers every redirect and DNS rebinding.
//
// With a proxy configured (WSAPI_HTTP_PROXY) the dial goes to the proxy, which
// is trusted operator config and may itself sit on a private address. The
// target host is then resolved and checked here before each request,
// redirects included; since the proxy resolves it again, DNS rebinding is only
// stopped if the proxy enforces its own egress rules.
//
// Keep-alives are off: these clients are built per download.
func NewPublicClient(timeout time.Duration, allowPrivate bool) *http.Client {
	var check addrCheck
	if !allowPrivate {
		check = requirePublic
	}
	return newPublicClient(timeout, proxyURL != nil, check)
}

func newPublicClient(timeout time.Duration, viaProxy bool, check addrCheck) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}

	var rt http.RoundTripper = tr
	if viaProxy {
		tr.Proxy = http.ProxyURL(proxyURL)
		if check != nil {
			rt = &resolveCheckTransport{base: tr, check: check}
		}
	} else if check != nil {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("parsing dial address %q: %w", address, err)
			}
			return check(ap)
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrUnsupportedScheme
			}
			return nil
		},
	}
}

// resolveCheckTransport checks every address the request host resolves to
// before handing the request to the proxy. Runs once per hop, so redirects
// are checked too.
type resolveCheckTransport struct {
	base  http.RoundTripper
	check addrCheck
}

func (t *resolveCheckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	port := req.URL.Port()
	if port == "" {
		port = "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: %w", port, err)
	}
	addrs, err := net.DefaultResolver.LookupNetIP(req.Context(), "ip", req.URL.Hostname())
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		if err := t.check(netip.AddrPortFrom(a, uint16(n))); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(req)
}
