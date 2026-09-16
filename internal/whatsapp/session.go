package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

// SessionService wraps the whatsmeow client for session management operations.
type SessionService struct {
	client         *whatsmeow.Client
	logger         *slog.Logger
	pairClientType string
	pairClientOS   string
}

// pairClientInfo holds the whatsmeow PairClientType, DeviceProps PlatformType,
// and the display name used in the "Browser (OS)" string for phone code pairing.
type pairClientInfo struct {
	pairType     whatsmeow.PairClientType
	platformType waCompanionReg.DeviceProps_PlatformType
	displayName  string
}

var pairClientMap = map[string]pairClientInfo{
	"chrome":  {whatsmeow.PairClientChrome, waCompanionReg.DeviceProps_CHROME, "Chrome"},
	"edge":    {whatsmeow.PairClientEdge, waCompanionReg.DeviceProps_EDGE, "Edge"},
	"firefox": {whatsmeow.PairClientFirefox, waCompanionReg.DeviceProps_FIREFOX, "Firefox"},
	"opera":   {whatsmeow.PairClientOpera, waCompanionReg.DeviceProps_OPERA, "Opera"},
	"safari":  {whatsmeow.PairClientSafari, waCompanionReg.DeviceProps_SAFARI, "Safari"},
}

func (s *SessionService) resolvePairClient() pairClientInfo {
	if info, ok := pairClientMap[strings.ToLower(s.pairClientType)]; ok {
		return info
	}
	return pairClientMap["chrome"]
}

// Connect establishes a connection to WhatsApp.
func (s *SessionService) Connect() error {
	return s.client.Connect()
}

// Disconnect disconnects from WhatsApp.
func (s *SessionService) Disconnect() {
	s.client.Disconnect()
}

// Logout disconnects from WhatsApp and removes the device session. The
// caller is responsible for discarding this *whatsmeow.Client afterwards
// (Manager.HandleLogout rebuilds the service) — calling Connect() on this
// client again would fail with "invalid use of deleted device".
func (s *SessionService) Logout(ctx context.Context) error {
	if err := s.client.Logout(ctx); err != nil {
		return fmt.Errorf("failed to logout: %v", err)
	}
	return nil
}

// qrWaitTimeout caps how long a single QR-generation request will wait for a
// `code` event from whatsmeow. Kept well below a typical HTTP client budget so
// we always return a clean response or a clean cancellation, not a race against
// a closing TCP connection.
const qrWaitTimeout = 30 * time.Second

// qrSessionMaxLifetime bounds the background drain below, which otherwise never
// returns: whatsmeow leaves the channel open when the client disconnects as
// expected, which is what every poll of this endpoint causes.
//
// It has to outlast any pairing that could still succeed, since a drain that
// gives up early hands the channel back to the deadlock it prevents. The bound
// is only here so that hammering the QR endpoint cannot pile up goroutines.
const qrSessionMaxLifetime = 45 * time.Minute

// drainQRChannel forwards the first item from src and then keeps consuming src
// until it closes (or maxLifetime elapses), discarding what it reads.
//
// The consuming is the point, not the forwarding. whatsmeow's qrChannel is a
// registered event handler, so it runs inside dispatchEvent holding a read lock
// on the handler list, and several of its branches send to this channel with a
// plain blocking send. Let one find a full buffer and it parks forever holding
// that read lock; the RemoveEventHandler it schedules then blocks on the write
// lock, and since Go's RWMutex hands off to a waiting writer before admitting
// readers, every later dispatchEvent on that client blocks behind it. The
// client stops handling nodes of every tag until the process restarts.
//
// One abandoned session cannot fill the buffer: WhatsApp sends about six refs
// per pair-device node and the buffer holds eight. It takes a second emitter,
// which is what polling produces: an expected disconnect emits no Disconnected
// event, so the previous poll's handler is never retired and starts emitting
// again after the reconnect. The risk tracks how many times the endpoint is
// polled before the scan lands.
func drainQRChannel(src <-chan whatsmeow.QRChannelItem, maxLifetime time.Duration) <-chan whatsmeow.QRChannelItem {
	first := make(chan whatsmeow.QRChannelItem, 1)

	go func() {
		defer close(first)

		// Deadline rather than a plain receive loop because whatsmeow's QR
		// emitter returns without closing the channel when the client
		// disconnects as expected — which is exactly what the next poll of
		// this endpoint does.
		expiry := time.NewTimer(maxLifetime)
		defer expiry.Stop()

		forwarded := false
		for {
			select {
			case item, ok := <-src:
				if !ok {
					return
				}
				if !forwarded {
					first <- item
					forwarded = true
				}
			case <-expiry.C:
				return
			}
		}
	}()

	return first
}

// GenerateQRImage generates a QR code image for WhatsApp Web login and returns
// the PNG bytes. Honors ctx for the wait — if the inbound HTTP request is
// cancelled the function returns promptly. The underlying QR pairing session
// is intentionally NOT tied to ctx: it must outlive a single HTTP request so
// the user can still scan a code returned to a previous poll.
func (s *SessionService) GenerateQRImage(ctx context.Context) ([]byte, error) {
	if s.client.Store.ID != nil {
		return nil, fmt.Errorf("device already registered")
	}

	// Disconnect any existing connection first.
	s.client.Disconnect()

	// Use Background, NOT ctx: tying the QR channel to the request context
	// caused whatsmeow to tear down the WhatsApp websocket the moment the
	// handler returned, so by the time the user scanned the QR PNG we just
	// sent back, the underlying ref token was already invalid.
	rawQRChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get QR channel: %v", err)
	}
	qrChan := drainQRChannel(rawQRChan, qrSessionMaxLifetime)

	go func() {
		if err := s.client.Connect(); err != nil {
			s.logger.Error("failed to connect for QR", "error", err)
		}
	}()

	select {
	case evt := <-qrChan:
		if evt.Event == "code" {
			qr, err := qrcode.New(evt.Code, qrcode.Medium)
			if err != nil {
				return nil, fmt.Errorf("failed to create QR code: %v", err)
			}
			png, err := qr.PNG(256)
			if err != nil {
				return nil, fmt.Errorf("failed to generate PNG: %v", err)
			}
			return png, nil
		}
		return nil, fmt.Errorf("unexpected QR event: %s", evt.Event)
	case <-ctx.Done():
		// Caller bailed — return promptly, but leave the WS alive: a
		// previous poll may have already shown the user a QR code they
		// are still about to scan.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: waiting for QR code", ErrTimeout)
		}
		return nil, ctx.Err()
	case <-time.After(qrWaitTimeout):
		// We genuinely waited for a code event with nothing to show for
		// it — release the underlying WS.
		s.client.Disconnect()
		return nil, fmt.Errorf("%w: waiting for QR code", ErrTimeout)
	}
}

// GenerateQRCode generates a QR code string for WhatsApp Web login. Lifetime
// semantics match GenerateQRImage: ctx governs only the wait, not the QR
// pairing session itself.
func (s *SessionService) GenerateQRCode(ctx context.Context) (string, error) {
	if s.client.Store.ID != nil {
		return "", fmt.Errorf("device already registered")
	}

	s.client.Disconnect()

	rawQRChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get QR channel: %v", err)
	}
	qrChan := drainQRChannel(rawQRChan, qrSessionMaxLifetime)

	go func() {
		if err := s.client.Connect(); err != nil {
			s.logger.Error("failed to connect for QR", "error", err)
		}
	}()

	select {
	case evt := <-qrChan:
		if evt.Event == "code" {
			return evt.Code, nil
		}
		return "", fmt.Errorf("unexpected QR event: %s", evt.Event)
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: waiting for QR code", ErrTimeout)
		}
		return "", ctx.Err()
	case <-time.After(qrWaitTimeout):
		s.client.Disconnect()
		return "", fmt.Errorf("%w: waiting for QR code", ErrTimeout)
	}
}

// GeneratePairCode generates a pair code for WhatsApp Web login using a phone
// number.
func (s *SessionService) GeneratePairCode(ctx context.Context, phone string) (string, error) {
	if !strings.HasPrefix(phone, "+") || len(phone) < 9 || len(phone) > 15 {
		return "", fmt.Errorf("invalid phone number. Should start with a + and have between 9 and 15 digits")
	}

	if s.client.Store.ID != nil {
		return "", fmt.Errorf("device already registered")
	}

	s.client.Disconnect()
	_ = s.client.Connect()

	// Allow the connection to establish before requesting a pair code.
	time.Sleep(3 * time.Second)

	info := s.resolvePairClient()
	displayName := info.displayName + " (" + s.pairClientOS + ")"
	return s.client.PairPhone(ctx, phone, true, info.pairType, displayName)
}

// IsConnected reports whether the client is connected to WhatsApp.
func (s *SessionService) IsConnected() bool {
	return s.client.IsConnected()
}

// IsLoggedIn reports whether the client has a paired device.
func (s *SessionService) IsLoggedIn() bool {
	if s.client.Store.ID == nil {
		return false
	}
	return s.client.IsLoggedIn()
}
