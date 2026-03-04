package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
)

// SessionService wraps the whatsmeow client for session management operations.
type SessionService struct {
	client                *whatsmeow.Client
	logger                *slog.Logger
	pairClientType        string
	pairClientDisplayName string
}

// resolvePairClientType maps a config string to the whatsmeow PairClientType.
func (s *SessionService) resolvePairClientType() whatsmeow.PairClientType {
	switch strings.ToLower(s.pairClientType) {
	case "edge":
		return whatsmeow.PairClientEdge
	case "firefox":
		return whatsmeow.PairClientFirefox
	case "opera":
		return whatsmeow.PairClientOpera
	case "safari":
		return whatsmeow.PairClientSafari
	default:
		return whatsmeow.PairClientChrome
	}
}

// Connect establishes a connection to WhatsApp.
func (s *SessionService) Connect() error {
	return s.client.Connect()
}

// Disconnect disconnects from WhatsApp.
func (s *SessionService) Disconnect() {
	s.client.Disconnect()
}

// Logout disconnects from WhatsApp and removes the device session.
func (s *SessionService) Logout(ctx context.Context) error {
	err := s.client.Logout(ctx)
	if err != nil {
		return fmt.Errorf("failed to logout: %v", err)
	}

	// Mark the store as uninitialized so that re-pairing triggers
	// initializeDevice (which creates sub-stores scoped to the new JID).
	// We must NOT replace cli.Store with a new *store.Device because the
	// appstate.Processor inside whatsmeow holds a direct pointer to it;
	// swapping the Store would leave the processor referencing the old
	// device, causing app state sync key lookups to use the wrong JID.
	s.client.Store.Initialized = false

	return nil
}

// GenerateQRImage generates a QR code image for WhatsApp Web login and returns
// the PNG bytes.
func (s *SessionService) GenerateQRImage() ([]byte, error) {
	if s.client.Store.ID != nil {
		return nil, fmt.Errorf("device already registered")
	}

	// Disconnect any existing connection first.
	s.client.Disconnect()

	qrChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get QR channel: %v", err)
	}

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
	case <-time.After(time.Minute):
		s.client.Disconnect()
		return nil, fmt.Errorf("timeout waiting for QR code")
	}
}

// GenerateQRCode generates a QR code string for WhatsApp Web login.
func (s *SessionService) GenerateQRCode() (string, error) {
	if s.client.Store.ID != nil {
		return "", fmt.Errorf("device already registered")
	}

	s.client.Disconnect()

	qrChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get QR channel: %v", err)
	}

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
	case <-time.After(time.Minute):
		s.client.Disconnect()
		return "", fmt.Errorf("timeout waiting for QR code")
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

	return s.client.PairPhone(ctx, phone, true, s.resolvePairClientType(), s.pairClientDisplayName)
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
