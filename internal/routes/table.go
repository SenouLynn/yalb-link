// Package routes maps MAVLink identities to inbound-observed links.
package routes

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"yalb.gcs/internal/codec"
)

// routeTTLMs keeps route reachability aligned with vehicle liveness.
const routeTTLMs = int64(codec.HeartbeatTTL / time.Millisecond)

// ErrNoRoute is returned when a target has no live inbound-derived route.
var ErrNoRoute = errors.New("routes: no live route to target")

// Key identifies a routable MAVLink component.
type Key struct {
	SysID  uint8
	CompID uint8
}

// String renders the key the way logs and errors refer to a target.
func (k Key) String() string {
	return fmt.Sprintf("%d:%d", k.SysID, k.CompID)
}

// Entry is one vehicle's return address.
type Entry struct {
	// Link is the codec-owned routable address.
	Link codec.LinkID
	// SrcIP and SrcPort are diagnostics, not send addresses.
	SrcIP   string
	SrcPort int

	LastSeenMs int64

	Key Key
}

// Table is a concurrency-safe route table with caller-driven expiry.
type Table struct {
	entries map[Key]Entry
	mu      sync.RWMutex
}

// NewTable returns an empty table.
func NewTable() *Table {
	return &Table{entries: make(map[Key]Entry)}
}

// Upsert records the latest link and preserves absent diagnostic addresses.
func (t *Table) Upsert(e Entry, nowMs int64) {
	e.LastSeenMs = nowMs

	t.mu.Lock()
	defer t.mu.Unlock()

	if prev, ok := t.entries[e.Key]; ok && e.SrcIP == "" {
		e.SrcIP, e.SrcPort = prev.SrcIP, prev.SrcPort
	}

	t.entries[e.Key] = e
}

// Lookup returns a live route and lazily removes that key if stale.
func (t *Table) Lookup(key Key, nowMs int64) (Entry, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e, ok := t.entries[key]
	if !ok {
		return Entry{}, false
	}

	if expired(e, nowMs) {
		delete(t.entries, key)

		return Entry{}, false
	}

	return e, true
}

// Resolve returns a target link or ErrNoRoute.
func (t *Table) Resolve(key Key, nowMs int64) (codec.LinkID, error) {
	e, ok := t.Lookup(key, nowMs)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoRoute, key)
	}

	return e.Link, nil
}

// Evict removes all stale routes.
func (t *Table) Evict(nowMs int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	var n int

	for key, e := range t.entries {
		if expired(e, nowMs) {
			delete(t.entries, key)

			n++
		}
	}

	return n
}

// ForgetLink drops every route pointing at a closed link.
func (t *Table) ForgetLink(link codec.LinkID) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	var n int

	for key, e := range t.entries {
		if e.Link == link {
			delete(t.entries, key)

			n++
		}
	}

	return n
}

// Len reports how many routes are held, including any not yet evicted.
func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.entries)
}

// Snapshot returns the current entries sorted by key, for diagnostics.
func (t *Table) Snapshot() []Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := slices.Collect(maps.Values(t.entries))
	slices.SortFunc(out, func(a, b Entry) int {
		if a.Key.SysID != b.Key.SysID {
			return int(a.Key.SysID) - int(b.Key.SysID)
		}

		return int(a.Key.CompID) - int(b.Key.CompID)
	})

	return out
}

func expired(e Entry, nowMs int64) bool {
	return nowMs-e.LastSeenMs > routeTTLMs
}
