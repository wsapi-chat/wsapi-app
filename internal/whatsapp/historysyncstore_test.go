package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestHistoryStore(t *testing.T) *HistorySyncStore {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MigrateCustomTables(db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	return NewHistorySyncStore(db, "sqlite")
}

func TestForEachMessagesStreamsRowsInOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestHistoryStore(t)
	exp := time.Now().Add(time.Hour)
	for _, m := range []string{`[{"id":"1"}]`, `[{"id":"2"}]`, `[{"id":"3"}]`} {
		if err := s.Insert(ctx, "me", "chat", m, exp); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Insert(ctx, "me", "other", `[{"id":"x"}]`, exp); err != nil {
		t.Fatal(err)
	}

	var got []string
	if err := s.ForEachMessages(ctx, "me", "chat", func(m string) error {
		got = append(got, m)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != `[{"id":"1"}]` || got[2] != `[{"id":"3"}]` {
		t.Fatalf("rows = %v", got)
	}

	// GetMessages keeps its contract on top of the stream.
	all, err := s.GetMessages(ctx, "me", "chat")
	if err != nil || len(all) != 3 {
		t.Fatalf("GetMessages = %v, %v", all, err)
	}
}

func TestForEachMessagesStopsOnCallbackError(t *testing.T) {
	ctx := context.Background()
	s := newTestHistoryStore(t)
	exp := time.Now().Add(time.Hour)
	for _, m := range []string{`[]`, `[]`, `[]`} {
		if err := s.Insert(ctx, "me", "chat", m, exp); err != nil {
			t.Fatal(err)
		}
	}
	stop := errors.New("stop")
	calls := 0
	err := s.ForEachMessages(ctx, "me", "chat", func(string) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
