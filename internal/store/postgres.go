package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

var _ Store = (*postgresStore)(nil)

type postgresStore struct {
	db *sql.DB
}

func (s *postgresStore) SaveInstance(ctx context.Context, inst InstanceRecord) error {
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT(id) DO UPDATE SET
			api_key        = EXCLUDED.api_key,
			webhook_url    = EXCLUDED.webhook_url,
			signing_secret = EXCLUDED.signing_secret,
			event_filters  = EXCLUDED.event_filters,
			history_sync   = EXCLUDED.history_sync,
			updated_at     = EXCLUDED.updated_at
	`, inst.ID, inst.APIKey, inst.WebhookURL, inst.SigningSecret, filters, historySync, inst.CreatedAt.UTC(), now)
	if err != nil {
		return fmt.Errorf("save instance %s: %w", inst.ID, err)
	}
	return nil
}

func (s *postgresStore) GetInstance(ctx context.Context, id string) (InstanceRecord, error) {
	var rec InstanceRecord
	var loggedInAt, loggedOutAt sql.NullTime
	var historySync sql.NullBool

	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
		       history_sync, created_at, updated_at, logged_in_at, logged_out_at
		FROM instances WHERE id = $1
	`, id).Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
		&scanFilters{&rec.EventFilters}, &historySync, &rec.CreatedAt, &rec.UpdatedAt, &loggedInAt, &loggedOutAt)
	if err == sql.ErrNoRows {
		return rec, fmt.Errorf("instance %s not found", id)
	}
	if err != nil {
		return rec, fmt.Errorf("get instance %s: %w", id, err)
	}
	if historySync.Valid {
		rec.HistorySync = &historySync.Bool
	}
	if loggedInAt.Valid {
		t := loggedInAt.Time.UTC()
		rec.LoggedInAt = &t
	}
	if loggedOutAt.Valid {
		t := loggedOutAt.Time.UTC()
		rec.LoggedOutAt = &t
	}
	return rec, nil
}

func (s *postgresStore) ListInstances(ctx context.Context) ([]InstanceRecord, error) {
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
		var loggedInAt, loggedOutAt sql.NullTime
		var historySync sql.NullBool

		if err := rows.Scan(&rec.ID, &rec.DeviceID, &rec.APIKey, &rec.WebhookURL, &rec.SigningSecret,
			&scanFilters{&rec.EventFilters}, &historySync, &rec.CreatedAt, &rec.UpdatedAt, &loggedInAt, &loggedOutAt); err != nil {
			return nil, fmt.Errorf("scan instance row: %w", err)
		}
		if historySync.Valid {
			rec.HistorySync = &historySync.Bool
		}
		if loggedInAt.Valid {
			t := loggedInAt.Time.UTC()
			rec.LoggedInAt = &t
		}
		if loggedOutAt.Valid {
			t := loggedOutAt.Time.UTC()
			rec.LoggedOutAt = &t
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *postgresStore) DeleteInstance(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete instance %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %s not found", id)
	}
	return nil
}

func (s *postgresStore) UpdateDeviceState(ctx context.Context, id, deviceID string, loggedIn bool) error {
	var err error
	if loggedIn {
		_, err = s.db.ExecContext(ctx, `
			UPDATE instances SET device_id = $1, logged_in_at = NOW(),
			       logged_out_at = NULL, updated_at = NOW()
			WHERE id = $2
		`, deviceID, id)
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE instances SET device_id = '', logged_out_at = NOW(),
			       updated_at = NOW()
			WHERE id = $1
		`, id)
	}
	if err != nil {
		return fmt.Errorf("update device state %s: %w", id, err)
	}
	return nil
}

func (s *postgresStore) Close() error {
	return s.db.Close()
}

