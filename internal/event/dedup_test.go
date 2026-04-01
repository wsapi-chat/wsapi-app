package event

import (
	"testing"
	"time"
)

func TestDedup_NewID(t *testing.T) {
	d := NewDedup(time.Hour)
	defer d.Close()

	if d.Contains("msg1") {
		t.Fatal("expected first call to return false (new ID)")
	}
}

func TestDedup_DuplicateID(t *testing.T) {
	d := NewDedup(time.Hour)
	defer d.Close()

	d.Contains("msg1")
	if !d.Contains("msg1") {
		t.Fatal("expected second call to return true (duplicate)")
	}
}

func TestDedup_DifferentIDs(t *testing.T) {
	d := NewDedup(time.Hour)
	defer d.Close()

	d.Contains("msg1")
	if d.Contains("msg2") {
		t.Fatal("expected different ID to return false")
	}
}

func TestDedup_ExpiredEntry(t *testing.T) {
	d := NewDedup(50 * time.Millisecond)
	defer d.Close()

	d.Contains("msg1")
	time.Sleep(60 * time.Millisecond)

	// Manually trigger cleanup since the ticker interval is 10 minutes.
	d.mu.Lock()
	cutoff := time.Now().Add(-d.ttl)
	for id, ts := range d.entries {
		if ts.Before(cutoff) {
			delete(d.entries, id)
		}
	}
	d.mu.Unlock()

	if d.Contains("msg1") {
		t.Fatal("expected expired ID to return false (treated as new)")
	}
}
