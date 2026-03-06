package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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

// InstanceStore manages the wsapi_instances table.
type InstanceStore struct {
	db      *sql.DB
	dialect string
}

// NewInstanceStore creates an InstanceStore using the given shared database pool.
// MigrateCustomTables must be called before this.
func NewInstanceStore(db *sql.DB, dialect string) *InstanceStore {
	return &InstanceStore{db: db, dialect: dialect}
}

func (s *InstanceStore) SaveInstance(ctx context.Context, inst InstanceRecord) error {
	filters := strings.Join(inst.EventFilters, ",")

	var historySync sql.NullBool
	if inst.HistorySync != nil {
		historySync = sql.NullBool{Bool: *inst.HistorySync, Valid: true}
	}

	q := `
		INSERT INTO wsapi_instances (id, api_key, webhook_url, signing_secret, event_filters, history_sync)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			api_key        = excluded.api_key,
			webhook_url    = excluded.webhook_url,
			signing_secret = excluded.signing_secret,
			event_filters  = excluded.event_filters,
			history_sync   = excluded.history_sync,
			updated_at     = CURRENT_TIMESTAMP
	`
	if s.dialect == dialectPostgres {
		q = `
			INSERT INTO wsapi_instances (id, api_key, webhook_url, signing_secret, event_filters, history_sync)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT(id) DO UPDATE SET
				api_key        = EXCLUDED.api_key,
				webhook_url    = EXCLUDED.webhook_url,
				signing_secret = EXCLUDED.signing_secret,
				event_filters  = EXCLUDED.event_filters,
				history_sync   = EXCLUDED.history_sync,
				updated_at     = NOW()
		`
	}

	_, err := s.db.ExecContext(ctx, q, inst.ID, inst.APIKey, inst.WebhookURL, inst.SigningSecret, filters, historySync)
	if err != nil {
		return fmt.Errorf("save instance %s: %w", inst.ID, err)
	}
	return nil
}

func (s *InstanceStore) GetInstance(ctx context.Context, id string) (InstanceRecord, error) {
	q := `
		SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
		       history_sync, created_at, updated_at, logged_in_at, logged_out_at
		FROM wsapi_instances WHERE id = ?
	`
	if s.dialect == dialectPostgres {
		q = `
			SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
			       history_sync, created_at, updated_at, logged_in_at, logged_out_at
			FROM wsapi_instances WHERE id = $1
		`
	}

	var rec InstanceRecord
	var historySync sql.NullBool

	if s.dialect == dialectPostgres {
		var loggedInAt, loggedOutAt sql.NullTime
		err := s.db.QueryRowContext(ctx, q, id).Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
			&scanFilters{&rec.EventFilters}, &historySync, &rec.CreatedAt, &rec.UpdatedAt, &loggedInAt, &loggedOutAt)
		if err == sql.ErrNoRows {
			return rec, fmt.Errorf("instance %s not found", id)
		}
		if err != nil {
			return rec, fmt.Errorf("get instance %s: %w", id, err)
		}
		if loggedInAt.Valid {
			t := loggedInAt.Time.UTC()
			rec.LoggedInAt = &t
		}
		if loggedOutAt.Valid {
			t := loggedOutAt.Time.UTC()
			rec.LoggedOutAt = &t
		}
	} else {
		var filters string
		var createdAt, updatedAt string
		var loggedInAt, loggedOutAt sql.NullString
		err := s.db.QueryRowContext(ctx, q, id).Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
			&filters, &historySync, &createdAt, &updatedAt, &loggedInAt, &loggedOutAt)
		if err == sql.ErrNoRows {
			return rec, fmt.Errorf("instance %s not found", id)
		}
		if err != nil {
			return rec, fmt.Errorf("get instance %s: %w", id, err)
		}
		rec.EventFilters = splitFilters(filters)
		rec.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
		rec.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
		if loggedInAt.Valid {
			t, _ := time.Parse(time.DateTime, loggedInAt.String)
			rec.LoggedInAt = &t
		}
		if loggedOutAt.Valid {
			t, _ := time.Parse(time.DateTime, loggedOutAt.String)
			rec.LoggedOutAt = &t
		}
	}

	if historySync.Valid {
		rec.HistorySync = &historySync.Bool
	}
	return rec, nil
}

func (s *InstanceStore) ListInstances(ctx context.Context) ([]InstanceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
		       history_sync, created_at, updated_at, logged_in_at, logged_out_at
		FROM wsapi_instances ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []InstanceRecord
	for rows.Next() {
		var rec InstanceRecord
		var historySync sql.NullBool

		if s.dialect == dialectPostgres {
			var loggedInAt, loggedOutAt sql.NullTime
			if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
				&scanFilters{&rec.EventFilters}, &historySync, &rec.CreatedAt, &rec.UpdatedAt, &loggedInAt, &loggedOutAt); err != nil {
				return nil, fmt.Errorf("scan instance row: %w", err)
			}
			if loggedInAt.Valid {
				t := loggedInAt.Time.UTC()
				rec.LoggedInAt = &t
			}
			if loggedOutAt.Valid {
				t := loggedOutAt.Time.UTC()
				rec.LoggedOutAt = &t
			}
		} else {
			var filters string
			var createdAt, updatedAt string
			var loggedInAt, loggedOutAt sql.NullString
			if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
				&filters, &historySync, &createdAt, &updatedAt, &loggedInAt, &loggedOutAt); err != nil {
				return nil, fmt.Errorf("scan instance row: %w", err)
			}
			rec.EventFilters = splitFilters(filters)
			rec.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
			rec.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
			if loggedInAt.Valid {
				t, _ := time.Parse(time.DateTime, loggedInAt.String)
				rec.LoggedInAt = &t
			}
			if loggedOutAt.Valid {
				t, _ := time.Parse(time.DateTime, loggedOutAt.String)
				rec.LoggedOutAt = &t
			}
		}

		if historySync.Valid {
			rec.HistorySync = &historySync.Bool
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *InstanceStore) DeleteInstance(ctx context.Context, id string) error {
	q := `DELETE FROM wsapi_instances WHERE id = ?`
	if s.dialect == dialectPostgres {
		q = `DELETE FROM wsapi_instances WHERE id = $1`
	}
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete instance %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %s not found", id)
	}
	return nil
}

func (s *InstanceStore) UpdateDeviceState(ctx context.Context, id, deviceID string, loggedIn bool) error {
	var q string
	var args []any

	if loggedIn {
		if s.dialect == dialectPostgres {
			q = `UPDATE wsapi_instances SET device_id = $1, logged_in_at = NOW(), logged_out_at = NULL, updated_at = NOW() WHERE id = $2`
		} else {
			q = `UPDATE wsapi_instances SET device_id = ?, logged_in_at = CURRENT_TIMESTAMP, logged_out_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
		}
		args = []any{deviceID, id}
	} else {
		if s.dialect == dialectPostgres {
			q = `UPDATE wsapi_instances SET device_id = '', logged_out_at = NOW(), updated_at = NOW() WHERE id = $1`
		} else {
			q = `UPDATE wsapi_instances SET device_id = '', logged_out_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
		}
		args = []any{id}
	}

	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update device state %s: %w", id, err)
	}
	return nil
}

// splitFilters splits a comma-separated filter string into a slice.
func splitFilters(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// scanFilters implements sql.Scanner to deserialize a comma-separated string
// into a []string slice during row scanning.
type scanFilters struct {
	dest *[]string
}

func (sf *scanFilters) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*sf.dest = splitFilters(v)
	case []byte:
		*sf.dest = splitFilters(string(v))
	case nil:
		*sf.dest = nil
	default:
		return fmt.Errorf("scanFilters: unsupported type %T", src)
	}
	return nil
}
