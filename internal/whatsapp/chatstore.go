package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ChatRecord tracks a known chat JID for a device.
type ChatRecord struct {
	OurJID       string
	ChatJID      string
	IsGroup      bool
	LastActivity time.Time
}

// ChatStore manages the wsapi_chats table in the whatsmeow database.
// It uses the same our_jid FK pattern as whatsmeow so that chat records
// are automatically purged when a device is deleted or logged out.
type ChatStore struct {
	db      *sql.DB
	dialect string // "sqlite" or "postgres"
}

// NewChatStore creates a ChatStore using the given shared database pool.
// MigrateCustomTables must be called before this.
func NewChatStore(db *sql.DB, dialect string) *ChatStore {
	return &ChatStore{db: db, dialect: dialect}
}

func (s *ChatStore) Upsert(ctx context.Context, rec ChatRecord) error {
	var lastActivity *time.Time
	if !rec.LastActivity.IsZero() {
		t := rec.LastActivity.UTC()
		lastActivity = &t
	}

	q := `
		INSERT INTO wsapi_chats (our_jid, chat_jid, is_group, last_activity)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(our_jid, chat_jid) DO UPDATE SET
			is_group      = excluded.is_group,
			last_activity = COALESCE(excluded.last_activity, wsapi_chats.last_activity)
	`
	args := []any{rec.OurJID, rec.ChatJID, rec.IsGroup, lastActivity}

	if s.dialect == "postgres" {
		q = `
			INSERT INTO wsapi_chats (our_jid, chat_jid, is_group, last_activity)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT(our_jid, chat_jid) DO UPDATE SET
				is_group      = EXCLUDED.is_group,
				last_activity = COALESCE(EXCLUDED.last_activity, wsapi_chats.last_activity)
		`
	}

	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("upsert chat %s: %w", rec.ChatJID, err)
	}
	return nil
}

func (s *ChatStore) Delete(ctx context.Context, ourJID, chatJID string) error {
	q := `DELETE FROM wsapi_chats WHERE our_jid = ? AND chat_jid = ?`
	if s.dialect == "postgres" {
		q = `DELETE FROM wsapi_chats WHERE our_jid = $1 AND chat_jid = $2`
	}
	_, err := s.db.ExecContext(ctx, q, ourJID, chatJID)
	if err != nil {
		return fmt.Errorf("delete chat %s: %w", chatJID, err)
	}
	return nil
}

func (s *ChatStore) List(ctx context.Context, ourJID string) ([]ChatRecord, error) {
	q := `
		SELECT our_jid, chat_jid, is_group, last_activity
		FROM wsapi_chats WHERE our_jid = ?
		ORDER BY last_activity DESC NULLS LAST
	`
	if s.dialect == "postgres" {
		q = `
			SELECT our_jid, chat_jid, is_group, last_activity
			FROM wsapi_chats WHERE our_jid = $1
			ORDER BY last_activity DESC NULLS LAST
		`
	}

	rows, err := s.db.QueryContext(ctx, q, ourJID)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []ChatRecord
	for rows.Next() {
		var rec ChatRecord
		var lastActivity sql.NullTime
		if err := rows.Scan(&rec.OurJID, &rec.ChatJID, &rec.IsGroup, &lastActivity); err != nil {
			return nil, fmt.Errorf("scan chat row: %w", err)
		}
		if lastActivity.Valid {
			rec.LastActivity = lastActivity.Time
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *ChatStore) Get(ctx context.Context, ourJID, chatJID string) (ChatRecord, error) {
	q := `
		SELECT our_jid, chat_jid, is_group, last_activity
		FROM wsapi_chats WHERE our_jid = ? AND chat_jid = ?
	`
	if s.dialect == "postgres" {
		q = `
			SELECT our_jid, chat_jid, is_group, last_activity
			FROM wsapi_chats WHERE our_jid = $1 AND chat_jid = $2
		`
	}

	var rec ChatRecord
	var lastActivity sql.NullTime
	err := s.db.QueryRowContext(ctx, q, ourJID, chatJID).Scan(&rec.OurJID, &rec.ChatJID, &rec.IsGroup, &lastActivity)
	if err != nil {
		return rec, fmt.Errorf("get chat %s: %w", chatJID, err)
	}
	if lastActivity.Valid {
		rec.LastActivity = lastActivity.Time
	}
	return rec, nil
}

