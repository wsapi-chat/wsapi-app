package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

var _ Store = (*sqliteStore)(nil)

type sqliteStore struct {
	db *sql.DB
}

func (s *sqliteStore) SaveInstance(ctx context.Context, inst InstanceRecord) error {
	filters := strings.Join(inst.EventFilters, ",")
	now := time.Now().UTC()
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}

	var historySync sql.NullBool
	if inst.HistorySync != nil {
		historySync = sql.NullBool{Bool: *inst.HistorySync, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO instances (id, api_key, webhook_url, signing_secret, event_filters, history_sync, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			api_key        = excluded.api_key,
			webhook_url    = excluded.webhook_url,
			signing_secret = excluded.signing_secret,
			event_filters  = excluded.event_filters,
			history_sync   = excluded.history_sync,
			updated_at     = excluded.updated_at
	`, inst.ID, inst.APIKey, inst.WebhookURL, inst.SigningSecret, filters, historySync, inst.CreatedAt.UTC(), now)
	if err != nil {
		return fmt.Errorf("save instance %s: %w", inst.ID, err)
	}
	return nil
}

func (s *sqliteStore) GetInstance(ctx context.Context, id string) (InstanceRecord, error) {
	var rec InstanceRecord
	var filters string
	var createdAt, updatedAt string
	var loggedInAt, loggedOutAt sql.NullString
	var historySync sql.NullBool

	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
		       history_sync, created_at, updated_at, logged_in_at, logged_out_at
		FROM instances WHERE id = ?
	`, id).Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
		&filters, &historySync, &createdAt, &updatedAt, &loggedInAt, &loggedOutAt)
	if err == sql.ErrNoRows {
		return rec, fmt.Errorf("instance %s not found", id)
	}
	if err != nil {
		return rec, fmt.Errorf("get instance %s: %w", id, err)
	}

	rec.EventFilters = splitFilters(filters)
	if historySync.Valid {
		rec.HistorySync = &historySync.Bool
	}
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
	return rec, nil
}

func (s *sqliteStore) ListInstances(ctx context.Context) ([]InstanceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
		       history_sync, created_at, updated_at, logged_in_at, logged_out_at
		FROM instances ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var records []InstanceRecord
	for rows.Next() {
		var rec InstanceRecord
		var filters string
		var createdAt, updatedAt string
		var loggedInAt, loggedOutAt sql.NullString
		var historySync sql.NullBool

		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
			&filters, &historySync, &createdAt, &updatedAt, &loggedInAt, &loggedOutAt); err != nil {
			return nil, fmt.Errorf("scan instance row: %w", err)
		}

		rec.EventFilters = splitFilters(filters)
		if historySync.Valid {
			rec.HistorySync = &historySync.Bool
		}
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
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *sqliteStore) DeleteInstance(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete instance %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %s not found", id)
	}
	return nil
}

func (s *sqliteStore) UpdateDeviceState(ctx context.Context, id, deviceID string, loggedIn bool) error {
	var err error
	if loggedIn {
		_, err = s.db.ExecContext(ctx, `
			UPDATE instances SET device_id = ?, logged_in_at = CURRENT_TIMESTAMP,
			       logged_out_at = NULL, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, deviceID, id)
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE instances SET device_id = '', logged_out_at = CURRENT_TIMESTAMP,
			       updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, id)
	}
	if err != nil {
		return fmt.Errorf("update device state %s: %w", id, err)
	}
	return nil
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

