package instance

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/wsapi-chat/wsapi-app/internal/whatsapp"
)

// Concurrency cover for the narrowed m.mu scope. The other delete tests are
// sequential, so -race over them proves nothing about the lock change.
//
// Run with: CGO_ENABLED=1 go test -race -count=10 ./internal/instance/...
func TestDeleteConcurrentWithLookups(t *testing.T) {
	ctx := context.Background()
	m, store := newDeleteTestManager(t)

	const n = 60
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("ins_race_%02d", i)
		if err := store.SaveInstance(ctx, whatsapp.InstanceRecord{ID: ids[i]}); err != nil {
			t.Fatalf("save: %v", err)
		}
		m.instances[ids[i]] = &Instance{ID: ids[i], Logger: m.logger}
	}

	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(4)
		// deleter
		go func() { defer wg.Done(); _ = m.DeleteInstance(ctx, id) }()
		// Do NOT assert on inst.deleting here: the field is guarded by m.mu and
		// the getters release it before returning, so reading it from the caller
		// is itself a race. TestLookupsSkipDeletingInstance covers the contract;
		// these goroutines only drive the paths concurrently.
		go func() { defer wg.Done(); _, _ = m.GetInstanceDirect(id) }()
		// second reader on the other lookup path
		go func() { defer wg.Done(); _, _ = m.GetInstance(id) }()
		// list walks the whole map while it is being mutated
		go func() { defer wg.Done(); _ = m.ListInstances() }()
	}
	wg.Wait()
}

// No entry may be stranded with the marker set, and no row may survive: that
// stranded state is what this change exists to make unreachable.
func TestConcurrentDeletesLeaveNoStrandedMarker(t *testing.T) {
	ctx := context.Background()
	m, store := newDeleteTestManager(t)

	const n = 40
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ins_dup_%02d", i)
		if err := store.SaveInstance(ctx, whatsapp.InstanceRecord{ID: id}); err != nil {
			t.Fatalf("save: %v", err)
		}
		m.instances[id] = &Instance{ID: id, Logger: m.logger}
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ins_dup_%02d", i)
		// two deleters for the SAME id, which is what a retry does
		for r := 0; r < 2; r++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = m.DeleteInstance(ctx, id) }()
		}
	}
	wg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()
	for id, inst := range m.instances {
		if inst.deleting {
			t.Errorf("%s left with the deleting marker set", id)
		}
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ins_dup_%02d", i)
		if _, err := store.GetInstance(ctx, id); err == nil {
			t.Errorf("%s row survived concurrent deletes", id)
		}
	}
}
