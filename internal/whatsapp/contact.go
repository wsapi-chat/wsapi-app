package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	waTypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"google.golang.org/protobuf/proto"
)

// ContactInfo is the domain response type for contact information.
type ContactInfo struct {
	identity.Identity
	FullName           string `json:"fullName"`
	FirstName          string `json:"firstName,omitempty"`
	PushName           string `json:"pushName"`
	BusinessName       string `json:"businessName,omitempty"`
	InPhoneAddressBook bool   `json:"inPhoneAddressBook"`
}

// ContactService wraps the whatsmeow client for contact operations.
type ContactService struct {
	client       *whatsmeow.Client
	logger       *slog.Logger
	contactStore *ContactStore
}

func (c *ContactService) getOurJID() string {
	if c.client.Store.ID == nil {
		return ""
	}
	return c.client.Store.ID.String()
}

// GetAllContacts returns all saved contacts from the custom contact store,
// enriched with push name and business name from whatsmeow's built-in store.
func (c *ContactService) GetAllContacts(ctx context.Context) ([]ContactInfo, error) {
	records, err := c.contactStore.List(ctx, c.getOurJID())
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %v", err)
	}

	lids := c.client.Store.LIDs
	contacts := make([]ContactInfo, 0, len(records))
	for _, rec := range records {
		jid, parseErr := parseJID(rec.ContactJID)
		if parseErr != nil {
			continue
		}

		ci := ContactInfo{
			Identity:           identity.Resolve(ctx, jid, waTypes.EmptyJID, lids),
			FullName:           rec.FullName,
			FirstName:          rec.FirstName,
			InPhoneAddressBook: rec.InPhoneAddressBook,
		}

		// Enrich with push name and business name from whatsmeow's built-in store.
		if info, infoErr := c.client.Store.Contacts.GetContact(ctx, jid); infoErr == nil {
			ci.PushName = info.PushName
			ci.BusinessName = info.BusinessName
		}

		contacts = append(contacts, ci)
	}

	return contacts, nil
}

// CreateOrUpdateContact creates or updates a contact via WhatsApp app state sync.
func (c *ContactService) CreateOrUpdateContact(ctx context.Context, contactID string, fullName string, firstName string) error {
	jid := FormatRecipient(contactID)

	mutation := appstate.MutationInfo{
		Index: []string{appstate.IndexContact, jid.String()},
		Value: &waSyncAction.SyncActionValue{
			ContactAction: &waSyncAction.ContactAction{
				FullName:  proto.String(fullName),
				FirstName: proto.String(firstName),
			},
		},
	}

	patch := appstate.PatchInfo{
		Type:      appstate.WAPatchCriticalUnblockLow,
		Mutations: []appstate.MutationInfo{mutation},
	}

	if err := c.client.SendAppState(context.Background(), patch); err != nil {
		return fmt.Errorf("failed to save contact: %v", err)
	}

	return nil
}

// SyncContacts triggers a full contact sync from the WhatsApp server.
func (c *ContactService) SyncContacts(ctx context.Context) error {
	return c.client.FetchAppState(ctx, appstate.WAPatchCriticalUnblockLow, true, false)
}

// GetContact retrieves information about a specific contact.
// It reads from the custom contact store first, falling back to whatsmeow's
// built-in store if the contact is not found.
func (c *ContactService) GetContact(ctx context.Context, contactID string) (ContactInfo, error) {
	jid, err := parseJID(contactID)
	if err != nil {
		return ContactInfo{}, fmt.Errorf("invalid contact JID: %v", err)
	}

	lids := c.client.Store.LIDs

	// Try custom contact store first.
	rec, recErr := c.contactStore.Get(ctx, c.getOurJID(), contactID)
	if recErr == nil {
		ci := ContactInfo{
			Identity:           identity.Resolve(ctx, jid, waTypes.EmptyJID, lids),
			FullName:           rec.FullName,
			FirstName:          rec.FirstName,
			InPhoneAddressBook: rec.InPhoneAddressBook,
		}
		if info, infoErr := c.client.Store.Contacts.GetContact(ctx, jid); infoErr == nil {
			ci.PushName = info.PushName
			ci.BusinessName = info.BusinessName
		}
		return ci, nil
	}
	if !errors.Is(recErr, sql.ErrNoRows) {
		c.logger.Warn("failed to get contact from store", "contactId", contactID, "error", recErr)
	}

	// Fall back to whatsmeow's built-in store.
	info, err := c.client.Store.Contacts.GetContact(ctx, jid)
	if err != nil {
		c.logger.Warn("failed to get contact info", "contactId", contactID, "error", err)
		return ContactInfo{}, nil
	}

	return ContactInfo{
		Identity:     identity.Resolve(ctx, jid, waTypes.EmptyJID, lids),
		FullName:     info.FullName,
		PushName:     info.PushName,
		BusinessName: info.BusinessName,
	}, nil
}

// BlockContact blocks a contact.
func (c *ContactService) BlockContact(ctx context.Context, contactID string) error {
	jid, err := parseJID(contactID)
	if err != nil {
		return fmt.Errorf("invalid contact JID: %w", err)
	}
	_, err = c.client.UpdateBlocklist(ctx, jid, events.BlocklistChangeActionBlock)
	return err
}

// UnblockContact unblocks a contact.
func (c *ContactService) UnblockContact(ctx context.Context, contactID string) error {
	jid, err := parseJID(contactID)
	if err != nil {
		return fmt.Errorf("invalid contact JID: %w", err)
	}
	_, err = c.client.UpdateBlocklist(ctx, jid, events.BlocklistChangeActionUnblock)
	return err
}

// GetBlocklist returns the list of blocked contacts.
func (c *ContactService) GetBlocklist(ctx context.Context) ([]identity.Identity, error) {
	blocklist, err := c.client.GetBlocklist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocklist: %w", err)
	}
	lids := c.client.Store.LIDs
	result := make([]identity.Identity, 0, len(blocklist.JIDs))
	for _, jid := range blocklist.JIDs {
		result = append(result, identity.Resolve(ctx, jid, waTypes.EmptyJID, lids))
	}
	return result, nil
}
