package instance

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/wsapi-chat/wsapi-app/internal/config"
	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// newDeleteTestManager returns a Manager backed by a fresh in-memory store,
// with no WhatsApp client: DeleteInstance only needs the map and the store.
func newDeleteTestManager(t *testing.T) (*Manager, *whatsapp.InstanceStore) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := whatsapp.MigrateCustomTables(db, "sqlite"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := whatsapp.NewInstanceStore(db, "sqlite")
	return &Manager{
		instances: make(map[string]*Instance),
		store:     store,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, store
}

// A row with no map entry is what a failed delete leaves behind. Deleting it
// must clear the row instead of returning at the map lookup, or the row is
// unreachable for good.
func TestDeleteInstanceClearsRowWithNoMapEntry(t *testing.T) {
	ctx := context.Background()
	m, store := newDeleteTestManager(t)

	if err := store.SaveInstance(ctx, whatsapp.InstanceRecord{ID: "ins_orphan"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := m.DeleteInstance(ctx, "ins_orphan"); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}

	if _, err := store.GetInstance(ctx, "ins_orphan"); err == nil {
		t.Fatal("row still present after delete")
	}
}

// Deleting something that is gone from both places is the end state the caller
// asked for, so it must not report failure and stop a retry from converging.
func TestDeleteInstanceIsIdempotent(t *testing.T) {
	m, _ := newDeleteTestManager(t)

	if err := m.DeleteInstance(context.Background(), "ins_missing"); err != nil {
		t.Fatalf("expected success on absent instance, got %v", err)
	}
}

// The map entry must outlive a failed store delete: it is the only handle the
// next attempt has. A closed DB is the cheapest way to force that failure.
func TestDeleteInstanceKeepsMapEntryWhenStoreFails(t *testing.T) {
	ctx := context.Background()
	m, _ := newDeleteTestManager(t)

	m.instances["ins_keep"] = &Instance{ID: "ins_keep", Logger: m.logger}

	db, err := sql.Open("sqlite", "file:closed?mode=memory")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_ = db.Close()
	m.store = whatsapp.NewInstanceStore(db, "sqlite")

	if err := m.DeleteInstance(ctx, "ins_keep"); err == nil {
		t.Fatal("expected the store failure to surface")
	}
	if _, ok := m.instances["ins_keep"]; !ok {
		t.Fatal("map entry dropped despite the store delete failing")
	}
}

func TestStoreDeleteReportsErrInstanceNotFound(t *testing.T) {
	_, store := newDeleteTestManager(t)

	err := store.DeleteInstance(context.Background(), "ins_absent")
	if !errors.Is(err, whatsapp.ErrInstanceNotFound) {
		t.Fatalf("want ErrInstanceNotFound, got %v", err)
	}
}

// A delete in progress must hide the instance: its service is being torn down,
// so handing it to a request would use a dead client.
func TestLookupsSkipDeletingInstance(t *testing.T) {
	m, _ := newDeleteTestManager(t)
	m.instances["ins_going"] = &Instance{ID: "ins_going", Logger: m.logger, deleting: true}

	if _, ok := m.GetInstanceDirect("ins_going"); ok {
		t.Fatal("GetInstanceDirect returned an instance that is being deleted")
	}
	if _, ok := m.GetInstance("ins_going"); ok {
		t.Fatal("GetInstance returned an instance that is being deleted")
	}
}

// A caller may read "already exists" as "adopt it", so returning that while a
// delete is running would hand back an instance about to be dropped.
func TestCreateDuringDeleteDoesNotReportAlreadyExists(t *testing.T) {
	m, _ := newDeleteTestManager(t)
	m.instances["ins_going"] = &Instance{ID: "ins_going", Logger: m.logger, deleting: true}

	_, err := m.CreateInstance(context.Background(), "ins_going", config.InstanceConfig{})
	if err == nil {
		t.Fatal("expected create to fail while a delete is in progress")
	}
	if strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error must not say 'already exists', got %v", err)
	}
}

// A failed delete must clear the marker, or the instance can never be deleted
// again — the same permanent state this work exists to remove.
func TestFailedDeleteClearsDeletingMarker(t *testing.T) {
	ctx := context.Background()
	m, _ := newDeleteTestManager(t)
	m.instances["ins_keep"] = &Instance{ID: "ins_keep", Logger: m.logger}

	db, err := sql.Open("sqlite", "file:closed-marker?mode=memory")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_ = db.Close()
	m.store = whatsapp.NewInstanceStore(db, "sqlite")

	if err := m.DeleteInstance(ctx, "ins_keep"); err == nil {
		t.Fatal("expected the store failure to surface")
	}
	if m.instances["ins_keep"].deleting {
		t.Fatal("deleting marker left set after a failed delete")
	}
	if err := m.DeleteInstance(ctx, "ins_keep"); err == nil || strings.Contains(err.Error(), "being deleted") {
		t.Fatalf("retry must attempt the delete again, got %v", err)
	}
}
