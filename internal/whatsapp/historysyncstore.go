package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// HistorySyncStore manages the wsapi_history_sync_messages table in the
// whatsmeow database. It caches projected history sync messages during
// initial pairing so they can be flushed on demand.
type HistorySyncStore struct {
	db      *sql.DB
	dialect string // "sqlite3" or "postgres"
}

// OpenHistorySyncStore opens a connection to the whatsmeow database for the
// wsapi_history_sync_messages table. MigrateCustomTables must be called before this.
func OpenHistorySyncStore(driver, dsn string) (*HistorySyncStore, error) {
	switch driver {
	case "sqlite":
		driver = "sqlite3"
	case "postgresql":
		driver = "postgres"
	}

	if driver == "sqlite3" && !strings.Contains(dsn, "_foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_foreign_keys=on"
		} else {
			dsn += "?_foreign_keys=on"
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open history sync store: %w", err)
	}

	return &HistorySyncStore{db: db, dialect: driver}, nil
}

// Insert adds a cached history sync message row. It also lazily deletes
// expired rows for the same our_jid before inserting.
func (s *HistorySyncStore) Insert(ctx context.Context, ourJID, chatJID, messagesJSON string, expireAt time.Time) error {
	// Lazily clean up expired rows for this device.
	delQ := `DELETE FROM wsapi_history_sync_messages WHERE our_jid = ? AND expire_at <= ?`
	if s.dialect == "postgres" {
		delQ = `DELETE FROM wsapi_history_sync_messages WHERE our_jid = $1 AND expire_at <= $2`
	}
	_, _ = s.db.ExecContext(ctx, delQ, ourJID, time.Now().UTC())

	q := `
		INSERT INTO wsapi_history_sync_messages (our_jid, chat_jid, messages, expire_at)
		VALUES (?, ?, ?, ?)
	`
	if s.dialect == "postgres" {
		q = `
			INSERT INTO wsapi_history_sync_messages (our_jid, chat_jid, messages, expire_at)
			VALUES ($1, $2, $3, $4)
		`
	}

	_, err := s.db.ExecContext(ctx, q, ourJID, chatJID, messagesJSON, expireAt.UTC())
	if err != nil {
		return fmt.Errorf("insert history sync messages for %s: %w", chatJID, err)
	}
	return nil
}

// ListChats returns the distinct chat JIDs that have cached (non-expired)
// history sync messages for the given device.
func (s *HistorySyncStore) ListChats(ctx context.Context, ourJID string) ([]string, error) {
	q := `
		SELECT DISTINCT chat_jid FROM wsapi_history_sync_messages
		WHERE our_jid = ? AND expire_at > ?
		ORDER BY chat_jid
	`
	if s.dialect == "postgres" {
		q = `
			SELECT DISTINCT chat_jid FROM wsapi_history_sync_messages
			WHERE our_jid = $1 AND expire_at > $2
			ORDER BY chat_jid
		`
	}

	rows, err := s.db.QueryContext(ctx, q, ourJID, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("list history sync chats: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var chats []string
	for rows.Next() {
		var chatJID string
		if err := rows.Scan(&chatJID); err != nil {
			return nil, fmt.Errorf("scan history sync chat: %w", err)
		}
		chats = append(chats, chatJID)
	}
	return chats, rows.Err()
}

// GetMessages returns the cached JSON message arrays for a specific chat,
// ordered by insertion order. Only non-expired rows are returned.
func (s *HistorySyncStore) GetMessages(ctx context.Context, ourJID, chatJID string) ([]string, error) {
	q := `
		SELECT messages FROM wsapi_history_sync_messages
		WHERE our_jid = ? AND chat_jid = ? AND expire_at > ?
		ORDER BY id
	`
	if s.dialect == "postgres" {
		q = `
			SELECT messages FROM wsapi_history_sync_messages
			WHERE our_jid = $1 AND chat_jid = $2 AND expire_at > $3
			ORDER BY id
		`
	}

	rows, err := s.db.QueryContext(ctx, q, ourJID, chatJID, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("get history sync messages for %s: %w", chatJID, err)
	}
	defer rows.Close() //nolint:errcheck

	var msgs []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("scan history sync message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// DeleteAll removes all cached history sync rows for the given device.
func (s *HistorySyncStore) DeleteAll(ctx context.Context, ourJID string) error {
	q := `DELETE FROM wsapi_history_sync_messages WHERE our_jid = ?`
	if s.dialect == "postgres" {
		q = `DELETE FROM wsapi_history_sync_messages WHERE our_jid = $1`
	}
	_, err := s.db.ExecContext(ctx, q, ourJID)
	if err != nil {
		return fmt.Errorf("delete history sync messages: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *HistorySyncStore) Close() error {
	return s.db.Close()
}
