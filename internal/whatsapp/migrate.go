package whatsapp

import (
	"database/sql"
	"fmt"
	"strings"

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
	{description: "create wsapi_chats table", migrate: waMigrateV1},
	{description: "create wsapi_contacts table", migrate: waMigrateV2},
	{description: "create wsapi_history_sync_messages table", migrate: waMigrateV3},
}

// MigrateCustomTables runs all pending schema migrations for WSAPI custom
// tables (wsapi_chats, wsapi_contacts, etc.) in the whatsmeow database.
// It must be called before opening ChatStore or ContactStore.
func MigrateCustomTables(driver, dsn string) error {
	if driver == dialectSQLite && !strings.Contains(dsn, "foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)"
		} else {
			dsn += "?_pragma=foreign_keys(1)"
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("open whatsmeow db for migration: %w", err)
	}
	defer db.Close() //nolint:errcheck

	return runWAMigrations(db, driver)
}

func runWAMigrations(db *sql.DB, dialect string) error {
	// Create version tracking table.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wsapi_wa_version (version INTEGER NOT NULL)`)
	if err != nil {
		return fmt.Errorf("create wa version table: %w", err)
	}

	// Read current version.
	var version int
	err = db.QueryRow(`SELECT version FROM wsapi_wa_version`).Scan(&version)
	if err == sql.ErrNoRows {
		// Determine starting point: if wsapi_chats already exists (from the
		// old raw-DDL init), treat V1 as already applied.
		if tableExists(db, dialect, "wsapi_chats") {
			version = 1
		} else {
			version = 0
		}
		_, err = db.Exec(fmt.Sprintf(`INSERT INTO wsapi_wa_version (version) VALUES (%d)`, version))
		if err != nil {
			return fmt.Errorf("insert initial wa version: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read wa schema version: %w", err)
	}

	// Run pending migrations.
	for i := version; i < len(allWAMigrations); i++ {
		if err := allWAMigrations[i].migrate(db, dialect); err != nil {
			return fmt.Errorf("wa migration %d (%s): %w", i+1, allWAMigrations[i].description, err)
		}
		_, err = db.Exec(fmt.Sprintf(`UPDATE wsapi_wa_version SET version = %d`, i+1))
		if err != nil {
			return fmt.Errorf("update wa version to %d: %w", i+1, err)
		}
	}

	return nil
}

// tableExists checks whether the given table exists in the database.
func tableExists(db *sql.DB, dialect, table string) bool {
	var query string
	switch dialect {
	case dialectSQLite:
		query = `SELECT 1 FROM sqlite_master WHERE type='table' AND name='` + table + `'`
	case dialectPostgres:
		query = `SELECT 1 FROM information_schema.tables WHERE table_name='` + table + `'`
	default:
		return false
	}
	var n int
	return db.QueryRow(query).Scan(&n) == nil
}

// waMigrateV1 creates the wsapi_chats table.
func waMigrateV1(db *sql.DB, dialect string) error {
	ddl := `
		CREATE TABLE IF NOT EXISTS wsapi_chats (
			our_jid       TEXT NOT NULL,
			chat_jid      TEXT NOT NULL,
			is_group      BOOLEAN NOT NULL DEFAULT 0,
			last_activity DATETIME,
			PRIMARY KEY (our_jid, chat_jid),
			FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)
	`
	if dialect == dialectPostgres {
		ddl = `
			CREATE TABLE IF NOT EXISTS wsapi_chats (
				our_jid       TEXT NOT NULL,
				chat_jid      TEXT NOT NULL,
				is_group      BOOLEAN NOT NULL DEFAULT false,
				last_activity TIMESTAMPTZ,
				PRIMARY KEY (our_jid, chat_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)
		`
	}
	_, err := db.Exec(ddl)
	return err
}

// waMigrateV3 creates the wsapi_history_sync_messages table for caching
// projected history sync messages during initial pairing.
func waMigrateV3(db *sql.DB, dialect string) error {
	ddl := `
		CREATE TABLE IF NOT EXISTS wsapi_history_sync_messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			our_jid    TEXT NOT NULL,
			chat_jid   TEXT NOT NULL,
			messages   TEXT NOT NULL,
			expire_at  DATETIME NOT NULL,
			FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)
	`
	if dialect == dialectPostgres {
		ddl = `
			CREATE TABLE IF NOT EXISTS wsapi_history_sync_messages (
				id         SERIAL PRIMARY KEY,
				our_jid    TEXT NOT NULL,
				chat_jid   TEXT NOT NULL,
				messages   TEXT NOT NULL,
				expire_at  TIMESTAMPTZ NOT NULL,
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)
		`
	}
	_, err := db.Exec(ddl)
	return err
}

// waMigrateV2 creates the wsapi_contacts table.
func waMigrateV2(db *sql.DB, dialect string) error {
	ddl := `
		CREATE TABLE IF NOT EXISTS wsapi_contacts (
			our_jid              TEXT NOT NULL,
			contact_jid          TEXT NOT NULL,
			contact_lid          TEXT NOT NULL DEFAULT '',
			first_name           TEXT NOT NULL DEFAULT '',
			full_name            TEXT NOT NULL DEFAULT '',
			in_phone_addressbook BOOLEAN NOT NULL DEFAULT 0,
			PRIMARY KEY (our_jid, contact_jid),
			FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)
	`
	if dialect == dialectPostgres {
		ddl = `
			CREATE TABLE IF NOT EXISTS wsapi_contacts (
				our_jid              TEXT NOT NULL,
				contact_jid          TEXT NOT NULL,
				contact_lid          TEXT NOT NULL DEFAULT '',
				first_name           TEXT NOT NULL DEFAULT '',
				full_name            TEXT NOT NULL DEFAULT '',
				in_phone_addressbook BOOLEAN NOT NULL DEFAULT false,
				PRIMARY KEY (our_jid, contact_jid),
				FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
			)
		`
	}
	_, err := db.Exec(ddl)
	return err
}
