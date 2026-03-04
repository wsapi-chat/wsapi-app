package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ContactRecord tracks a saved contact for a device.
type ContactRecord struct {
	OurJID             string
	ContactJID         string
	ContactLID         string
	FirstName          string
	FullName           string
	InPhoneAddressBook bool
}

// ContactStore manages the wsapi_contacts table in the whatsmeow database.
// It uses the same our_jid FK pattern as whatsmeow so that contact records
// are automatically purged when a device is deleted or logged out.
type ContactStore struct {
	db      *sql.DB
	dialect string // "sqlite" or "postgres"
}

// OpenContactStore opens a connection to the whatsmeow database for the
// wsapi_contacts table. MigrateCustomTables must be called before this.
func OpenContactStore(driver, dsn string) (*ContactStore, error) {
	if driver == "sqlite" && !strings.Contains(dsn, "foreign_keys") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=foreign_keys(1)"
		} else {
			dsn += "?_pragma=foreign_keys(1)"
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open contact store: %w", err)
	}

	return &ContactStore{db: db, dialect: driver}, nil
}

func (s *ContactStore) Upsert(ctx context.Context, rec ContactRecord) error {
	q := `
		INSERT INTO wsapi_contacts (our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(our_jid, contact_jid) DO UPDATE SET
			contact_lid          = excluded.contact_lid,
			first_name           = excluded.first_name,
			full_name            = excluded.full_name,
			in_phone_addressbook = excluded.in_phone_addressbook
	`
	args := []any{rec.OurJID, rec.ContactJID, rec.ContactLID, rec.FirstName, rec.FullName, rec.InPhoneAddressBook}

	if s.dialect == "postgres" {
		q = `
			INSERT INTO wsapi_contacts (our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT(our_jid, contact_jid) DO UPDATE SET
				contact_lid          = EXCLUDED.contact_lid,
				first_name           = EXCLUDED.first_name,
				full_name            = EXCLUDED.full_name,
				in_phone_addressbook = EXCLUDED.in_phone_addressbook
		`
	}

	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("upsert contact %s: %w", rec.ContactJID, err)
	}
	return nil
}

func (s *ContactStore) Delete(ctx context.Context, ourJID, contactJID string) error {
	q := `DELETE FROM wsapi_contacts WHERE our_jid = ? AND contact_jid = ?`
	if s.dialect == "postgres" {
		q = `DELETE FROM wsapi_contacts WHERE our_jid = $1 AND contact_jid = $2`
	}
	_, err := s.db.ExecContext(ctx, q, ourJID, contactJID)
	if err != nil {
		return fmt.Errorf("delete contact %s: %w", contactJID, err)
	}
	return nil
}

func (s *ContactStore) List(ctx context.Context, ourJID string) ([]ContactRecord, error) {
	q := `
		SELECT our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook
		FROM wsapi_contacts WHERE our_jid = ?
		ORDER BY full_name
	`
	if s.dialect == "postgres" {
		q = `
			SELECT our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook
			FROM wsapi_contacts WHERE our_jid = $1
			ORDER BY full_name
		`
	}

	rows, err := s.db.QueryContext(ctx, q, ourJID)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []ContactRecord
	for rows.Next() {
		var rec ContactRecord
		if err := rows.Scan(&rec.OurJID, &rec.ContactJID, &rec.ContactLID, &rec.FirstName, &rec.FullName, &rec.InPhoneAddressBook); err != nil {
			return nil, fmt.Errorf("scan contact row: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *ContactStore) Get(ctx context.Context, ourJID, contactJID string) (ContactRecord, error) {
	q := `
		SELECT our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook
		FROM wsapi_contacts WHERE our_jid = ? AND contact_jid = ?
	`
	if s.dialect == "postgres" {
		q = `
			SELECT our_jid, contact_jid, contact_lid, first_name, full_name, in_phone_addressbook
			FROM wsapi_contacts WHERE our_jid = $1 AND contact_jid = $2
		`
	}

	var rec ContactRecord
	err := s.db.QueryRowContext(ctx, q, ourJID, contactJID).Scan(
		&rec.OurJID, &rec.ContactJID, &rec.ContactLID, &rec.FirstName, &rec.FullName, &rec.InPhoneAddressBook,
	)
	if err != nil {
		return rec, fmt.Errorf("get contact %s: %w", contactJID, err)
	}
	return rec, nil
}

func (s *ContactStore) Close() error {
	return s.db.Close()
}
