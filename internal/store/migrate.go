package store

import (
	"database/sql"
	"fmt"
)

const dialectSQLite = "sqlite"
const dialectPostgres = "postgres"

type migration struct {
	description string
	migrate     func(db *sql.DB, dialect string) error
}

var allMigrations = []migration{
	{description: "initial schema", migrate: migrateV1},
	{description: "add device state timestamps", migrate: migrateV2},
	{description: "add history_sync column", migrate: migrateV3},
	{description: "make history_sync nullable", migrate: migrateV4},
}

// migrate runs all pending schema migrations against the database.
// It creates a wsapi_version table to track the current schema version
// and detects pre-migration databases (instances table exists but no version table).
func migrate(db *sql.DB, dialect string) error {
	// Create version tracking table.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wsapi_version (version INTEGER NOT NULL)`)
	if err != nil {
		return fmt.Errorf("create version table: %w", err)
	}

	// Read current version.
	var version int
	err = db.QueryRow(`SELECT version FROM wsapi_version`).Scan(&version)
	if err == sql.ErrNoRows {
		// No version row yet — determine starting point.
		if tableExists(db, dialect, "instances") {
			// Pre-migration database: instances table exists from before
			// the migration system. Assume it matches the v1 schema.
			version = 1
		} else {
			// Fresh database: no tables exist yet.
			version = 0
		}
		_, err = db.Exec(fmt.Sprintf(`INSERT INTO wsapi_version (version) VALUES (%d)`, version))
		if err != nil {
			return fmt.Errorf("insert initial version: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	// Run pending migrations.
	for i := version; i < len(allMigrations); i++ {
		if err := allMigrations[i].migrate(db, dialect); err != nil {
			return fmt.Errorf("migration %d (%s): %w", i+1, allMigrations[i].description, err)
		}
		_, err = db.Exec(fmt.Sprintf(`UPDATE wsapi_version SET version = %d`, i+1))
		if err != nil {
			return fmt.Errorf("update version to %d: %w", i+1, err)
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

// migrateV1 creates the initial instances table with the original 7 columns.
func migrateV1(db *sql.DB, dialect string) error {
	var ts string
	switch dialect {
	case dialectSQLite:
		ts = "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP"
	case dialectPostgres:
		ts = "TIMESTAMPTZ NOT NULL DEFAULT NOW()"
	}

	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS instances (
			id             TEXT PRIMARY KEY,
			device_id      TEXT NOT NULL DEFAULT '',
			api_key        TEXT NOT NULL DEFAULT '',
			webhook_url    TEXT NOT NULL DEFAULT '',
			signing_secret TEXT NOT NULL DEFAULT '',
			event_filters  TEXT NOT NULL DEFAULT '',
			created_at     %s
		)
	`, ts))
	return err
}

// migrateV2 adds the updated_at, logged_in_at, and logged_out_at columns.
func migrateV2(db *sql.DB, dialect string) error {
	type colDef struct {
		name    string
		typeDef string
	}

	var cols []colDef
	switch dialect {
	case dialectSQLite:
		cols = []colDef{
			{"updated_at", "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP"},
			{"logged_in_at", "DATETIME"},
			{"logged_out_at", "DATETIME"},
		}
	case dialectPostgres:
		cols = []colDef{
			{"updated_at", "TIMESTAMPTZ NOT NULL DEFAULT NOW()"},
			{"logged_in_at", "TIMESTAMPTZ"},
			{"logged_out_at", "TIMESTAMPTZ"},
		}
	}

	for _, col := range cols {
		var stmt string
		switch dialect {
		case dialectPostgres:
			stmt = fmt.Sprintf("ALTER TABLE instances ADD COLUMN IF NOT EXISTS %s %s", col.name, col.typeDef)
		default:
			stmt = fmt.Sprintf("ALTER TABLE instances ADD COLUMN %s %s", col.name, col.typeDef)
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("add column %s: %w", col.name, err)
		}
	}
	return nil
}

// migrateV3 adds the history_sync column to the instances table.
func migrateV3(db *sql.DB, dialect string) error {
	var stmt string
	switch dialect {
	case dialectPostgres:
		stmt = "ALTER TABLE instances ADD COLUMN IF NOT EXISTS history_sync BOOLEAN NOT NULL DEFAULT false"
	default:
		stmt = "ALTER TABLE instances ADD COLUMN history_sync BOOLEAN NOT NULL DEFAULT 0"
	}
	_, err := db.Exec(stmt)
	return err
}

// migrateV4 makes history_sync nullable, converting false (0) to NULL.
func migrateV4(db *sql.DB, dialect string) error {
	switch dialect {
	case dialectSQLite:
		// SQLite does not support ALTER COLUMN; recreate the table.
		stmts := []string{
			`CREATE TABLE instances_new (
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
			`INSERT INTO instances_new
				SELECT id, device_id, api_key, webhook_url, signing_secret, event_filters,
				       CASE WHEN history_sync = 0 THEN NULL ELSE history_sync END,
				       created_at, updated_at, logged_in_at, logged_out_at
				FROM instances`,
			`DROP TABLE instances`,
			`ALTER TABLE instances_new RENAME TO instances`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("sqlite migrate v4: %w", err)
			}
		}
		return nil

	case dialectPostgres:
		stmts := []string{
			`ALTER TABLE instances ALTER COLUMN history_sync DROP NOT NULL`,
			`ALTER TABLE instances ALTER COLUMN history_sync SET DEFAULT NULL`,
			`UPDATE instances SET history_sync = NULL WHERE history_sync = false`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("postgres migrate v4: %w", err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported dialect: %s", dialect)
	}
}
