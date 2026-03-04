package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Store provides persistence for instance records.
type Store interface {
	SaveInstance(ctx context.Context, inst InstanceRecord) error
	GetInstance(ctx context.Context, id string) (InstanceRecord, error)
	ListInstances(ctx context.Context) ([]InstanceRecord, error)
	DeleteInstance(ctx context.Context, id string) error
	UpdateDeviceState(ctx context.Context, id, deviceID string, loggedIn bool) error
	Close() error
}

// InstanceRecord is the persisted representation of a managed instance.
type InstanceRecord struct {
	ID            string     `json:"id"`
	DeviceID      string     `json:"deviceId"`
	APIKey        string     `json:"apiKey"`
	WebhookURL    string     `json:"webhookUrl"`
	SigningSecret string     `json:"signingSecret"`
	EventFilters  []string   `json:"eventFilters"`
	HistorySync   *bool      `json:"historySync"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	LoggedInAt    *time.Time `json:"loggedInAt,omitempty"`
	LoggedOutAt   *time.Time `json:"loggedOutAt,omitempty"`
}

// Open creates a Store backed by the given database driver.
// Supported drivers: "sqlite", "postgres".
func Open(driver, dsn string) (Store, error) {
	switch driver {
	case "sqlite":
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		if err := migrate(db, dialectSQLite); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate sqlite: %w", err)
		}
		return &sqliteStore{db: db}, nil

	case "postgres", "postgresql":
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		if err := migrate(db, dialectPostgres); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate postgres: %w", err)
		}
		return &postgresStore{db: db}, nil

	default:
		return nil, fmt.Errorf("unsupported database driver: %q", driver)
	}
}
