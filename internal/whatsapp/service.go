package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Service wraps a whatsmeow client and exposes domain-specific sub-services.
type Service struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	logger    *slog.Logger

	Messages    *MessageService
	Groups      *GroupService
	Communities *CommunityService
	Contacts    *ContactService
	Chats       *ChatService
	Account     *UserMeService
	Users       *UserService
	Media       *MediaService
	Session     *SessionService
	Calls       *CallService
	Newsletters *NewsletterService
	Status      *StatusService
	HistorySync *HistorySyncService
}

// NewService creates a whatsmeow-backed Service. If deviceID is non-empty the
// existing device store is loaded; otherwise a new device is created.
func NewService(ctx context.Context, container *sqlstore.Container, deviceID string, logger, waLogger *slog.Logger, chatStore *ChatStore, contactStore *ContactStore, historySyncStore *HistorySyncStore) (*Service, error) {
	var deviceStore *store.Device

	if deviceID != "" {
		did, err := parseJID(deviceID)
		if err != nil {
			return nil, fmt.Errorf("parse device ID: %w", err)
		}
		deviceStore, err = container.GetDevice(ctx, did)
		if err != nil {
			logger.Warn("device not found, creating new", "deviceId", deviceID, "error", err)
			deviceStore = container.NewDevice()
		}
		if deviceStore == nil {
			logger.Warn("device store is nil, creating new", "deviceId", deviceID)
			deviceStore = container.NewDevice()
		}
	} else {
		deviceStore = container.NewDevice()
	}

	store.SetOSInfo("Windows", store.GetWAVersion())
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()

	waClient := whatsmeow.NewClient(deviceStore, NewSlogAdapter(waLogger))
	waClient.EmitAppStateEventsOnFullSync = true

	svc := &Service{
		client:    waClient,
		container: container,
		logger:    logger,
	}

	svc.Messages = &MessageService{client: waClient, logger: logger}
	svc.Groups = &GroupService{client: waClient, logger: logger}
	svc.Communities = &CommunityService{client: waClient, logger: logger}
	svc.Contacts = &ContactService{client: waClient, logger: logger, contactStore: contactStore}
	svc.Chats = &ChatService{client: waClient, logger: logger, chatStore: chatStore}
	svc.Account = &UserMeService{client: waClient, logger: logger}
	svc.Users = &UserService{client: waClient, logger: logger}
	svc.Media = &MediaService{client: waClient, logger: logger}
	svc.Session = &SessionService{client: waClient, logger: logger, pairClientType: "chrome", pairClientDisplayName: "Chrome (Windows)"}
	svc.Calls = &CallService{client: waClient, logger: logger}
	svc.Newsletters = &NewsletterService{client: waClient, logger: logger}
	svc.Status = &StatusService{client: waClient, logger: logger}
	svc.HistorySync = &HistorySyncService{client: waClient, logger: logger, store: historySyncStore}

	return svc, nil
}

// SetPairClient configures the pair client type and display name used during
// phone-number pairing. Defaults to Chrome (Windows) if not called.
func (s *Service) SetPairClient(clientType, displayName string) {
	s.Session.pairClientType = clientType
	s.Session.pairClientDisplayName = displayName
}

// Client returns the underlying whatsmeow client.
func (s *Service) Client() *whatsmeow.Client {
	return s.client
}

// Connect establishes the WhatsApp connection.
func (s *Service) Connect() error {
	return s.client.Connect()
}

// Disconnect tears down the WhatsApp connection.
func (s *Service) Disconnect() {
	if s.client.IsConnected() {
		s.client.Disconnect()
	}
}

// IsConnected reports whether the client is connected.
func (s *Service) IsConnected() bool {
	return s.client.IsConnected()
}

// IsLoggedIn reports whether the client has a paired device.
func (s *Service) IsLoggedIn() bool {
	if s.client.Store.ID == nil {
		return false
	}
	return s.client.IsLoggedIn()
}

// GetDeviceJID returns the device JID string, or empty if not paired.
func (s *Service) GetDeviceJID() string {
	if s.client.Store.ID == nil {
		return ""
	}
	return s.client.Store.ID.String()
}

// AddEventHandler registers an event handler on the underlying whatsmeow client.
func (s *Service) AddEventHandler(handler whatsmeow.EventHandler) {
	s.client.AddEventHandler(handler)
}

// RemoveEventHandlers removes all event handlers.
func (s *Service) RemoveEventHandlers() {
	s.client.RemoveEventHandlers()
}

// DeleteDevice disconnects and removes the device from the store.
func (s *Service) DeleteDevice() {
	s.Disconnect()
	_ = s.client.Store.Delete(context.Background())
}

// OpenContainer initializes a whatsmeow sqlstore container.
func OpenContainer(ctx context.Context, driver, dsn string, logger *slog.Logger) (*sqlstore.Container, error) {
	// SQLite requires foreign keys enabled for whatsmeow schema migrations.
	if driver == "sqlite" && !strings.Contains(dsn, "foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)"
		} else {
			dsn += "?_pragma=foreign_keys(1)"
		}
	}

	container, err := sqlstore.New(ctx, driver, dsn, NewSlogAdapter(logger))
	if err != nil {
		return nil, fmt.Errorf("open whatsmeow store: %w", err)
	}
	return container, nil
}
