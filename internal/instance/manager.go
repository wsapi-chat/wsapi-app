package instance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/event"
	"github.com/wsapi-chat/wsapi-app/internal/publisher"
	"github.com/wsapi-chat/wsapi-app/internal/server/middleware"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// Manager manages the lifecycle of WhatsApp instances. It keeps a thread-safe
// in-memory map of active instances backed by persistent storage.
type Manager struct {
	mu               sync.RWMutex
	instances        map[string]*Instance
	store            *whatsapp.InstanceStore
	container        *sqlstore.Container
	chatStore        *whatsapp.ChatStore
	contactStore     *whatsapp.ContactStore
	historySyncStore *whatsapp.HistorySyncStore
	cfg              *config.Config
	pubFact          publisher.PublisherFactory
	logger           *slog.Logger
	waLogger         *slog.Logger
}

// NewManager creates a new instance manager.
func NewManager(st *whatsapp.InstanceStore, container *sqlstore.Container, chatStore *whatsapp.ChatStore, contactStore *whatsapp.ContactStore, historySyncStore *whatsapp.HistorySyncStore, cfg *config.Config, pubFactory publisher.PublisherFactory, logger, waLogger *slog.Logger) *Manager {
	return &Manager{
		instances:        make(map[string]*Instance),
		store:            st,
		container:        container,
		chatStore:        chatStore,
		contactStore:     contactStore,
		historySyncStore: historySyncStore,
		cfg:              cfg,
		pubFact:          pubFactory,
		logger:           logger,
		waLogger:         waLogger,
	}
}

// GetInstance returns the instance with the given ID. It satisfies the
// middleware.InstanceResolver interface so the middleware can look up
// instances and validate their API keys.
func (m *Manager) GetInstance(id string) (middleware.Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	if !ok {
		return nil, false
	}
	return inst, true
}

// GetInstanceDirect returns the concrete *Instance (for use outside the
// middleware resolution path).
func (m *Manager) GetInstanceDirect(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	return inst, ok
}

// ListInstances returns a snapshot of all active instances.
func (m *Manager) ListInstances() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst)
	}
	return out
}

// applyDefaults fills empty/nil fields in cfg from global defaults.
func (m *Manager) applyDefaults(cfg *config.InstanceConfig) {
	d := &m.cfg.InstanceDefaults
	if cfg.APIKey == "" {
		cfg.APIKey = d.APIKey
	}
	if cfg.WebhookURL == "" {
		cfg.WebhookURL = d.WebhookURL
	}
	if cfg.SigningSecret == "" {
		cfg.SigningSecret = d.SigningSecret
	}
	if len(cfg.EventFilters) == 0 {
		cfg.EventFilters = d.EventFilters
	}
	if cfg.HistorySync == nil {
		cfg.HistorySync = d.HistorySync
	}
}

// logConfigResolution logs which fields use global defaults vs instance overrides.
func (m *Manager) logConfigResolution(action, id string, cfg *config.InstanceConfig) {
	d := &m.cfg.InstanceDefaults
	var defaultFields, instanceFields []string

	if cfg.WebhookURL == "" || cfg.WebhookURL == d.WebhookURL {
		if d.WebhookURL != "" {
			defaultFields = append(defaultFields, "webhookUrl")
		}
	} else {
		instanceFields = append(instanceFields, "webhookUrl")
	}
	if cfg.SigningSecret == "" || cfg.SigningSecret == d.SigningSecret {
		if d.SigningSecret != "" {
			defaultFields = append(defaultFields, "signingSecret")
		}
	} else {
		instanceFields = append(instanceFields, "signingSecret")
	}
	if len(cfg.EventFilters) == 0 {
		if len(d.EventFilters) > 0 {
			defaultFields = append(defaultFields, "eventFilters")
		}
	} else {
		instanceFields = append(instanceFields, "eventFilters")
	}
	if cfg.APIKey != "" && cfg.APIKey != d.APIKey {
		instanceFields = append(instanceFields, "apiKey")
	} else if d.APIKey != "" {
		defaultFields = append(defaultFields, "apiKey")
	}
	if cfg.HistorySync != nil {
		instanceFields = append(instanceFields, "historySync")
	} else if d.HistorySync != nil {
		defaultFields = append(defaultFields, "historySync")
	}

	if len(defaultFields) > 0 {
		m.logger.Debug(action+" using global defaults", "id", id, "fields", defaultFields)
	}
	if len(instanceFields) > 0 {
		m.logger.Debug(action+" using overrides", "id", id, "fields", instanceFields)
	}
}

// CreateInstance creates a new instance, persists it, and registers it in memory.
func (m *Manager) CreateInstance(ctx context.Context, id string, cfg config.InstanceConfig) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; exists {
		return nil, fmt.Errorf("instance %s already exists", id)
	}

	m.logConfigResolution("instance create", id, &cfg)
	m.applyDefaults(&cfg)

	// Strip system events — they are always delivered and must not
	// appear in user-configured filter lists.
	cfg.EventFilters = event.StripSystemEvents(cfg.EventFilters)

	// Persist to store.
	rec := whatsapp.InstanceRecord{
		ID:            id,
		APIKey:        cfg.APIKey,
		WebhookURL:    cfg.WebhookURL,
		SigningSecret: cfg.SigningSecret,
		EventFilters:  cfg.EventFilters,
		HistorySync:   cfg.HistorySync,
	}
	if err := m.store.SaveInstance(ctx, rec); err != nil {
		return nil, fmt.Errorf("persist instance: %w", err)
	}

	inst := m.buildInstance(ctx, id, "", cfg)
	m.instances[id] = inst
	m.logger.Info("instance created", "id", id)
	return inst, nil
}

// DeleteInstance removes an instance from memory and persistent storage.
func (m *Manager) DeleteInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Log out from WhatsApp to unlink the device, then clean up.
	wasLoggedIn := false
	if inst.Service != nil {
		wasLoggedIn = inst.Service.IsLoggedIn()
		if wasLoggedIn {
			if err := inst.Service.Session.Logout(ctx); err != nil {
				inst.Logger.Warn("logout before delete failed, forcing disconnect", "error", err)
				inst.Service.DeleteDevice()
			}
		} else {
			inst.Service.DeleteDevice()
		}
	}

	// client.Logout() does not emit a LoggedOut event, so publish
	// explicitly before closing the publisher.
	if wasLoggedIn && inst.Publisher != nil {
		logoutEvt := event.NewEvent(id, event.TypeLoggedOut, event.SessionLoggedOutEvent{
			Reason: "instance_deleted",
		})
		if err := inst.Publisher.Publish(ctx, logoutEvt); err != nil {
			inst.Logger.Error("failed to publish logout event", "error", err)
		}
	}
	if inst.Publisher != nil {
		_ = inst.Publisher.Close()
	}
	if inst.Dedup != nil {
		inst.Dedup.Close()
	}

	delete(m.instances, id)

	if err := m.store.DeleteInstance(ctx, id); err != nil {
		return fmt.Errorf("delete instance from store: %w", err)
	}

	m.logger.Info("instance deleted", "id", id)
	return nil
}

// UpdateInstanceConfig updates the configuration of an existing instance,
// persists the change, and recreates its publisher.
func (m *Manager) UpdateInstanceConfig(ctx context.Context, id string, cfg config.InstanceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Close old publisher before replacing.
	if inst.Publisher != nil {
		_ = inst.Publisher.Close()
	}

	m.logConfigResolution("instance update", id, &cfg)
	m.applyDefaults(&cfg)

	// Strip system events — they are always delivered and must not
	// appear in user-configured filter lists.
	cfg.EventFilters = event.StripSystemEvents(cfg.EventFilters)

	inst.Config = cfg
	inst.Publisher = m.pubFact.Create(id, cfg.WebhookURL, cfg.SigningSecret)

	// Persist updated record.
	rec := whatsapp.InstanceRecord{
		ID:            id,
		APIKey:        cfg.APIKey,
		WebhookURL:    cfg.WebhookURL,
		SigningSecret: cfg.SigningSecret,
		EventFilters:  cfg.EventFilters,
		HistorySync:   cfg.HistorySync,
	}
	if err := m.store.SaveInstance(ctx, rec); err != nil {
		return fmt.Errorf("persist instance update: %w", err)
	}

	m.logger.Info("instance config updated", "id", id)
	return nil
}

// SingleInstanceID is the fixed instance ID used in single-instance mode.
const SingleInstanceID = "default"

// EnsureSingleInstance provisions or refreshes the "default" instance for
// single-instance mode. It uses whatsmeow's GetFirstDevice to resolve the
// device session directly, bypassing the wsapi store entirely.
func (m *Manager) EnsureSingleInstance(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use whatsmeow's GetFirstDevice — returns existing device or creates new.
	device, err := m.container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("get first device: %w", err)
	}

	var deviceID string
	if device.ID != nil {
		deviceID = device.ID.String()
	}

	// Build config entirely from current InstanceDefaults.
	cfg := m.cfg.InstanceDefaults
	cfg.EventFilters = event.StripSystemEvents(cfg.EventFilters)

	inst := m.buildInstance(ctx, SingleInstanceID, deviceID, cfg)
	m.instances[SingleInstanceID] = inst

	m.logger.Info("single instance ready", "id", SingleInstanceID)
	return nil
}

// RestoreInstances loads all persisted instance records and re-creates their
// in-memory representation. Intended to be called once at startup.
func (m *Manager) RestoreInstances(ctx context.Context) error {
	records, err := m.store.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("list instances from store: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rec := range records {
		cfg := config.InstanceConfig{
			APIKey:        rec.APIKey,
			WebhookURL:    rec.WebhookURL,
			SigningSecret: rec.SigningSecret,
			EventFilters:  rec.EventFilters,
			HistorySync:   rec.HistorySync,
		}

		m.applyDefaults(&cfg)

		inst := m.buildInstance(ctx, rec.ID, rec.DeviceID, cfg)
		m.instances[rec.ID] = inst
		m.logger.Info("instance restored", "id", rec.ID)
	}

	m.logger.Info("instance restore complete", "count", len(records))
	return nil
}

// RestartInstance tears down and re-creates an instance's runtime resources
// (publisher, future WhatsApp service) without changing its persisted config.
func (m *Manager) RestartInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Close old resources.
	if inst.Service != nil {
		inst.Service.Disconnect()
	}
	if inst.Publisher != nil {
		_ = inst.Publisher.Close()
	}
	if inst.Dedup != nil {
		inst.Dedup.Close()
	}

	// Rebuild with same config — use existing device JID if available.
	deviceID := ""
	if inst.Service != nil {
		deviceID = inst.Service.GetDeviceJID()
	}
	rebuilt := m.buildInstance(ctx, id, deviceID, inst.Config)
	m.instances[id] = rebuilt

	m.logger.Info("instance restarted", "id", id)
	return nil
}

// HandleLogout persists the logout state for an instance and publishes a
// logged_out event. This must be called when the user logs out via the API,
// because client.Logout() does not emit a LoggedOut event.
func (m *Manager) HandleLogout(ctx context.Context, id string) {
	if m.store != nil {
		if err := m.store.UpdateDeviceState(ctx, id, "", false); err != nil {
			m.logger.Error("failed to persist API logout state", "id", id, "error", err)
		}
	}

	m.mu.RLock()
	inst, ok := m.instances[id]
	m.mu.RUnlock()
	if ok && inst.Publisher != nil {
		logoutEvt := event.NewEvent(id, event.TypeLoggedOut, event.SessionLoggedOutEvent{
			Reason: "api_logout",
		})
		if err := inst.Publisher.Publish(ctx, logoutEvt); err != nil {
			m.logger.Error("failed to publish logout event", "id", id, "error", err)
		}
	}
}

// Shutdown gracefully closes all active instances and their resources.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, inst := range m.instances {
		if inst.Service != nil {
			inst.Service.Disconnect()
		}
		if inst.Publisher != nil {
			_ = inst.Publisher.Close()
		}
		if inst.Dedup != nil {
			inst.Dedup.Close()
		}
		m.logger.Info("instance shut down", "id", id)
	}

	m.instances = make(map[string]*Instance)
	m.logger.Info("all instances shut down")
}

// buildInstance creates an Instance struct from a config, including the
// WhatsApp service and event handler. deviceID may be empty for new instances.
// Must be called with m.mu held (or during init before the manager is shared).
func (m *Manager) buildInstance(ctx context.Context, id, deviceID string, cfg config.InstanceConfig) *Instance {
	instLogger := m.logger.With("instanceId", id)
	pub := m.pubFact.Create(id, cfg.WebhookURL, cfg.SigningSecret)

	inst := &Instance{
		ID:        id,
		Config:    cfg,
		Publisher: pub,
		Dedup:     event.NewDedup(2 * time.Hour),
		Logger:    instLogger,
	}

	if err := m.initService(ctx, inst, deviceID); err != nil {
		instLogger.Error("failed to create whatsapp service", "error", err)
	}

	return inst
}

// initService creates a new WhatsApp service for the given instance and
// registers the event handler. deviceID may be empty for new/unlinked instances.
// On success the instance's Service field is replaced with the new service.
func (m *Manager) initService(ctx context.Context, inst *Instance, deviceID string) error {
	id := inst.ID
	instLogger := inst.Logger
	pub := inst.Publisher
	cfg := inst.Config

	dedup := inst.Dedup

	waInstLogger := m.waLogger.With("instanceId", id)
	svc, err := whatsapp.NewService(ctx, m.container, deviceID, instLogger, waInstLogger, m.chatStore, m.contactStore, m.historySyncStore)
	if err != nil {
		return fmt.Errorf("create whatsapp service: %w", err)
	}
	svc.SetPairClient(m.cfg.Whatsmeow.PairClientType, m.cfg.Whatsmeow.PairClientOS)
	inst.Service = svc

	// Register the event handler that tracks chats and projects/publishes events.
	pctx := &event.ProjectorContext{Client: svc.Client(), InstanceID: id}
	chatStore := m.chatStore
	contactStore := m.contactStore
	historySyncStore := m.historySyncStore
	waClient := svc.Client()
	var appStateRecovering sync.Map // keyed by appstate.WAPatchName; prevents concurrent recovery per collection
	initialSyncPublished := &sync.Once{}
	historySyncEnabled := cfg.HistorySync != nil && *cfg.HistorySync
	svc.AddEventHandler(func(evt interface{}) {
		// Track chats from incoming events before projecting.
		// ourJID is resolved dynamically since it's nil until pairing completes.
		if ourJID := svc.GetDeviceJID(); ourJID != "" {
			switch e := evt.(type) {
			case *waEvents.HistorySync:
				if e.Data.SyncType != nil {
					syncType := *e.Data.SyncType
					if syncType == waHistorySync.HistorySync_INITIAL_BOOTSTRAP ||
						syncType == waHistorySync.HistorySync_FULL ||
						syncType == waHistorySync.HistorySync_RECENT {
						for _, conv := range e.Data.Conversations {
							if conv == nil {
								continue
							}
							chatJID, parseErr := waTypes.ParseJID(conv.GetID())
							if parseErr != nil {
								continue
							}
							// Resolve LIDs to phone-based JIDs to avoid duplicates.
							// Skip status broadcast and unresolvable LIDs.
							chatJID = resolveLID(chatJID, waClient)
							if chatJID.Server == waTypes.HiddenUserServer || skipChat(chatJID) {
								continue
							}
							rec := whatsapp.ChatRecord{
								OurJID:  ourJID,
								ChatJID: chatJID.String(),
								IsGroup: chatJID.Server == waTypes.GroupServer,
							}
							if ts := conv.GetConversationTimestamp(); ts > 0 {
								rec.LastActivity = time.Unix(int64(ts), 0)
							}
							_ = chatStore.Upsert(context.Background(), rec)
						}

						// Cache projected messages when history sync is enabled.
						if historySyncEnabled {
							cacheHistorySyncMessages(waClient, e, ourJID, historySyncStore, pctx, instLogger)

							// RECENT with progress >= 100 is the last message-carrying
							// history sync during initial pairing. Publish initial_sync_finished
							// once all messages have been cached so consumers can flush.
							if syncType == waHistorySync.HistorySync_RECENT && e.Data.GetProgress() >= 100 {
								initialSyncPublished.Do(func() {
									syncEvt := event.NewEvent(id, event.TypeInitialSyncFinished, event.InitialSyncFinishedEvent{})
									if err := pub.Publish(context.Background(), syncEvt); err != nil {
										instLogger.Error("failed to publish event", "type", syncEvt.EventType, "error", err)
									}
								})
							}
						}
					}
				}
			case *waEvents.Message:
				chatJID := resolveLID(e.Info.Chat, waClient)
				if chatJID.Server == waTypes.HiddenUserServer || skipChat(chatJID) {
					break
				}
				rec := whatsapp.ChatRecord{
					OurJID:       ourJID,
					ChatJID:      chatJID.String(),
					IsGroup:      chatJID.Server == waTypes.GroupServer,
					LastActivity: e.Info.Timestamp,
				}
				_ = chatStore.Upsert(context.Background(), rec)
			case *waEvents.DeleteChat:
				delJID := resolveLID(e.JID, waClient)
				if delJID.Server == waTypes.HiddenUserServer {
					break
				}
				_ = chatStore.Delete(context.Background(), ourJID, delJID.String())
			case *waEvents.Contact:
				if e.Action != nil {
					contactJID := resolveLID(e.JID, waClient)
					if contactJID.Server == waTypes.HiddenUserServer {
						break
					}
					rec := whatsapp.ContactRecord{
						OurJID:             ourJID,
						ContactJID:         contactJID.String(),
						FirstName:          e.Action.GetFirstName(),
						FullName:           e.Action.GetFullName(),
						InPhoneAddressBook: e.Action.GetSaveOnPrimaryAddressbook(),
					}
					if lidStr := e.Action.GetLidJID(); lidStr != "" {
						rec.ContactLID = lidStr
					}
					_ = contactStore.Upsert(context.Background(), rec)
				}
			}
		}

		// Track device state changes (login/logout).
		switch e := evt.(type) {
		case *waEvents.PairSuccess:
			// Reset the initial sync flag so a new pairing can fire the event again.
			initialSyncPublished = &sync.Once{}
			if m.store != nil {
				deviceJID := e.ID.String()
				if err := m.store.UpdateDeviceState(context.Background(), id, deviceJID, true); err != nil {
					instLogger.Error("failed to persist login state", "error", err)
				}
			}
		case *waEvents.LoggedOut:
			instLogger.Info("device logged out", "reason", e.Reason.String())
			if m.store != nil {
				if err := m.store.UpdateDeviceState(context.Background(), id, "", false); err != nil {
					instLogger.Error("failed to persist logout state", "error", err)
				}
			}
			// Publish logged_out directly rather than relying on the projector
			// pipeline below. During a 401-on-connect the handler queue may
			// shut down before Part 3 completes; publishing here guarantees
			// delivery. The return skips Part 3 to prevent double publish.
			logoutEvt := event.NewEvent(id, event.TypeLoggedOut, event.SessionLoggedOutEvent{
				Reason: e.Reason.String(),
			})
			if err := pub.Publish(context.Background(), logoutEvt); err != nil {
				instLogger.Error("failed to publish event", "type", logoutEvt.EventType, "error", err)
			}
			// Rebuild the service with a fresh device store so re-pairing
			// works without FK constraint violations. This runs async because
			// we are inside the old client's event handler goroutine.
			go m.rebuildInstanceService(id)
			return
		case *waEvents.AppStateSyncComplete:
			if e.Recovery {
				instLogger.Info("app state recovery completed", "name", string(e.Name), "version", e.Version)
			} else {
				instLogger.Info("app state sync completed", "name", string(e.Name), "version", e.Version)
			}
		case *waEvents.AppStateSyncError:
			instLogger.Warn("app state sync failed", "name", string(e.Name), "error", e.Error)
			if errors.Is(e.Error, appstate.ErrMismatchingLTHash) && !e.FullSync {
				go recoverAppState(waClient, e.Name, instLogger, &appStateRecovering)
			}
		}

		// Project and publish.
		projected, ok := event.Project(evt, pctx)
		if !ok {
			return
		}

		// Check event filters. System events (logged_in, logged_out,
		// logged_error, initial_sync_finished) are always delivered.
		if len(cfg.EventFilters) > 0 && !event.IsSystemEvent(projected.EventType) {
			matched := false
			for _, f := range cfg.EventFilters {
				if f == projected.EventType {
					matched = true
					break
				}
			}
			if !matched {
				return
			}
		}

		// Deduplicate message events by whatsmeow message ID to prevent
		// duplicate deliveries (e.g. unavailable message retry flow).
		if msg, ok := projected.Data.(event.MessageEvent); ok {
			if dedup.Contains(msg.ID) {
				instLogger.Debug("duplicate message event discarded", "messageId", msg.ID)
				return
			}
		}

		if err := pub.Publish(context.Background(), projected); err != nil {
			instLogger.Error("failed to publish event", "type", projected.EventType, "error", err)
		}
	})

	// Auto-connect if the device has been paired.
	if deviceID != "" {
		if err := svc.Connect(); err != nil {
			instLogger.Error("failed to connect whatsapp", "error", err)
		}
	}

	return nil
}

// rebuildInstanceService replaces the WhatsApp service on an existing instance
// with a fresh device store. Called after a server-side logout so re-pairing
// starts clean without FK constraint violations from stale device data.
func (m *Manager) rebuildInstanceService(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return
	}

	// Disconnect the old client before replacing.
	if inst.Service != nil {
		inst.Service.Disconnect()
	}

	if err := m.initService(context.Background(), inst, ""); err != nil {
		inst.Logger.Error("failed to rebuild service after logout", "error", err)
	} else {
		inst.Logger.Info("service rebuilt after server-side logout")
	}
}

// isBroadcast returns true if the JID is a broadcast JID (status broadcast
// or broadcast list), which should not be tracked as a chat.
func skipChat(jid waTypes.JID) bool {
	return jid.Server == waTypes.BroadcastServer || jid.User == "0"
}

// cacheHistorySyncMessages projects and caches history sync messages into the
// wsapi_history_sync_messages table for later flushing via the API.
func cacheHistorySyncMessages(client *whatsmeow.Client, e *waEvents.HistorySync, ourJID string, store *whatsapp.HistorySyncStore, pctx *event.ProjectorContext, logger *slog.Logger) {
	expireAt := time.Now().Add(1 * time.Hour)

	for _, conv := range e.Data.Conversations {
		if conv == nil {
			continue
		}

		chatJID, err := waTypes.ParseJID(conv.GetID())
		if err != nil {
			continue
		}

		chatJID = resolveLID(chatJID, client)
		if chatJID.Server == waTypes.HiddenUserServer || skipChat(chatJID) {
			continue
		}

		convMessages := conv.GetMessages()
		if len(convMessages) == 0 {
			continue
		}

		var messages []event.MessageEvent
		for _, msg := range convMessages {
			waMsg, err := client.ParseWebMessage(chatJID, msg.GetMessage())
			if err != nil {
				continue
			}

			_, data, publish := event.ProjectMessage(waMsg, pctx)
			if !publish {
				continue
			}

			msgEvt, ok := data.(event.MessageEvent)
			if !ok || msgEvt.Type == "unknown" {
				continue
			}

			messages = append(messages, msgEvt)
		}

		if len(messages) == 0 {
			continue
		}

		jsonBytes, err := json.Marshal(messages)
		if err != nil {
			logger.Error("history sync cache: marshal messages", "chatJid", chatJID.String(), "error", err)
			continue
		}

		if err := store.Insert(context.Background(), ourJID, chatJID.String(), string(jsonBytes), expireAt); err != nil {
			logger.Error("history sync cache: insert", "chatJid", chatJID.String(), "error", err)
		}
	}
}

// recoverAppState requests an unencrypted app state snapshot from the primary
// device via peer data recovery. This bypasses LTHash verification entirely,
// making it the most reliable recovery for hash-mismatch errors. The response
// is handled automatically by whatsmeow (handleAppStateRecovery) and emits
// AppStateSyncComplete{Recovery: true} on success.
//
// The recovering sync.Map ensures only one recovery is in-flight per collection
// — rapid-fire sync failures won't spawn duplicate goroutines.
func recoverAppState(client *whatsmeow.Client, name appstate.WAPatchName, logger *slog.Logger, recovering *sync.Map) {
	if _, loaded := recovering.LoadOrStore(name, true); loaded {
		logger.Debug("app state recovery already in progress, skipping", "name", string(name))
		return
	}
	defer recovering.Delete(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("requesting app state peer recovery", "name", string(name))
	msg := whatsmeow.BuildAppStateRecoveryRequest(name)
	_, err := client.SendPeerMessage(ctx, msg)
	if err != nil {
		logger.Error("failed to send app state recovery request", "name", string(name), "error", err)
		return
	}
	logger.Info("app state recovery request sent to primary device", "name", string(name))
}

// resolveLID converts a LID-based JID to a phone-based JID using the device's
// LID mapping store. If the JID is not a LID or resolution fails, the original
// JID is returned unchanged.
func resolveLID(jid waTypes.JID, client *whatsmeow.Client) waTypes.JID {
	if jid.Server != waTypes.HiddenUserServer {
		return jid
	}
	pn, err := client.Store.GetAltJID(context.Background(), jid)
	if err != nil || pn.User == "" {
		return jid
	}
	return pn
}
