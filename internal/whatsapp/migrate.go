package whatsapp

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

const (
	dialectSQLite   = "sqlite"
	dialectPostgres = "postgres"
)

type waMigration struct {
	description string
	migrate     func(db *sql.DB, dialect string) error
}

var allWAMigrations = []waMigration{
	{description: "create all wsapi tables", migrate: waMigrateV1},
}

// MigrateCustomTables runs all pending schema migrations for WSAPI custom
// tables in the database. It must be called before creating any stores.
func MigrateCustomTables(db *sql.DB, dialect string) error {
	return runWAMigrations(db, dialect)
}

func runWAMigrations(db *sql.DB, dialect string) error {
	// Create version tracking table.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wsapi_version (version INTEGER NOT NULL)`)
	if err != nil {
		return fmt.Errorf("create version table: %w", err)
	}

	// Read current version.
	var version int
	err = db.QueryRow(`SELECT version FROM wsapi_version`).Scan(&version)
	if err == sql.ErrNoRows {
		version = 0
		_, err = db.Exec(`INSERT INTO wsapi_version (version) VALUES (0)`)
		if err != nil {
			return fmt.Errorf("insert initial version: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	// Run pending migrations.
	for i := version; i < len(allWAMigrations); i++ {
		if err := allWAMigrations[i].migrate(db, dialect); err != nil {
			return fmt.Errorf("migration %d (%s): %w", i+1, allWAMigrations[i].description, err)
		}
		_, err = db.Exec(fmt.Sprintf(`UPDATE wsapi_version SET version = %d`, i+1))
		if err != nil {
			return fmt.Errorf("update version to %d: %w", i+1, err)
		}
	}

	return nil
}

// waMigrateV1 creates all WSAPI custom tables.
func waMigrateV1(db *sql.DB, dialect string) error {
	var stmts []string

	switch dialect {
	case dialectSQLite:
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS wsapi_chats (
				our_jid       TEXT NOT NULL,
				chat_jid      TEXT NOT NULL,
				is_group      BOOLEAN NOT NULL DEFAULT 0,
				last_activity DATETIME,
				PRIMARY KEY (our_jid, chat_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_contacts (
				our_jid              TEXT NOT NULL,
				contact_jid          TEXT NOT NULL,
				contact_lid          TEXT NOT NULL DEFAULT '',
				first_name           TEXT NOT NULL DEFAULT '',
				full_name            TEXT NOT NULL DEFAULT '',
				in_phone_addressbook BOOLEAN NOT NULL DEFAULT 0,
				PRIMARY KEY (our_jid, contact_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_history_sync_messages (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				our_jid    TEXT NOT NULL,
				chat_jid   TEXT NOT NULL,
				messages   TEXT NOT NULL,
				expire_at  DATETIME NOT NULL,
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_instances (
				id             TEXT PRIMARY KEY,
				device_id      TEXT NOT NULL DEFAULT '',
				api_key        TEXT NOT NULL DEFAULT '',
				webhook_url    TEXT NOT NULL DEFAULT '',
				signing_secret TEXT NOT NULL DEFAULT '',
				event_filters  TEXT NOT NULL DEFAULT '',
				history_sync   INTEGER DEFAULT NULL,
				created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				logged_in_at   DATETIME,
				logged_out_at  DATETIME
			)`,
		}
	case dialectPostgres:
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS wsapi_chats (
				our_jid       TEXT NOT NULL,
				chat_jid      TEXT NOT NULL,
				is_group      BOOLEAN NOT NULL DEFAULT false,
				last_activity TIMESTAMPTZ,
				PRIMARY KEY (our_jid, chat_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_contacts (
				our_jid              TEXT NOT NULL,
				contact_jid          TEXT NOT NULL,
				contact_lid          TEXT NOT NULL DEFAULT '',
				first_name           TEXT NOT NULL DEFAULT '',
				full_name            TEXT NOT NULL DEFAULT '',
				in_phone_addressbook BOOLEAN NOT NULL DEFAULT false,
				PRIMARY KEY (our_jid, contact_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_history_sync_messages (
				id         SERIAL PRIMARY KEY,
				our_jid    TEXT NOT NULL,
				chat_jid   TEXT NOT NULL,
				messages   TEXT NOT NULL,
				expire_at  TIMESTAMPTZ NOT NULL,
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS wsapi_instances (
				id             TEXT PRIMARY KEY,
				device_id      TEXT NOT NULL DEFAULT '',
				api_key        TEXT NOT NULL DEFAULT '',
				webhook_url    TEXT NOT NULL DEFAULT '',
				signing_secret TEXT NOT NULL DEFAULT '',
				event_filters  TEXT NOT NULL DEFAULT '',
				history_sync   BOOLEAN DEFAULT NULL,
				created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				logged_in_at   TIMESTAMPTZ,
				logged_out_at  TIMESTAMPTZ
			)`,
		}
	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}
