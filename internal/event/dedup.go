package event

import (
	"sync"
	"time"
)

// Dedup is a concurrent-safe set that tracks recently seen message IDs
// and automatically purges entries older than the configured TTL.
type Dedup struct {
	mu        sync.Mutex
	entries   map[string]time.Time
	ttl       time.Duration
	done      chan struct{}
	closeOnce sync.Once
}

// NewDedup creates a Dedup set with the given TTL and starts a background
// goroutine that periodically removes expired entries.
func NewDedup(ttl time.Duration) *Dedup {
	d := &Dedup{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		done:    make(chan struct{}),
	}
	go d.cleanup()
	return d
}

// Contains reports whether id has been seen within the TTL window.
// If id is new, it is recorded and false is returned.
// If id is already present (duplicate), true is returned.
func (d *Dedup) Contains(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.entries[id]; ok {
		return true
	}
	d.entries[id] = time.Now()
	return false
}

// Close stops the background cleanup goroutine. Safe to call multiple times.
func (d *Dedup) Close() {
	d.closeOnce.Do(func() { close(d.done) })
}

func (d *Dedup) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
			d.mu.Lock()
			cutoff := time.Now().Add(-d.ttl)
			for id, t := range d.entries {
				if t.Before(cutoff) {
					delete(d.entries, id)
				}
			}
			d.mu.Unlock()
		}
	}
}
