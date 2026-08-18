// Package routes maps a vehicle to the link it was last heard on.
//
// This is the send path's address book. It exists because MAVLink identity
// (system ID, component ID) and transport identity (which radio, which UDP
// peer) are different things, and the only place they are ever observed
// together is on an inbound frame. Nothing pre-registers a vehicle; a vehicle
// that has never been heard from is not addressable, and an unaddressable
// target is rejected rather than broadcast.
//
// Like the vehicle fold, this package takes time as a parameter. Eviction is
// the interesting behaviour and it is only testable with an injected clock.
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

// routeTTLMs matches the vehicle fold's heartbeat TTL. A route is a claim that
// a vehicle is reachable on a link, which is the same claim the fold makes
// when it says the vehicle is alive — two different answers to one question
// would surface as commands accepted for vehicles the UI shows as lost.
const routeTTLMs = int64(codec.HeartbeatTTL / time.Millisecond)

// ErrNoRoute is returned by Resolve when the target has no live route.
//
// This is the "reject, never broadcast" rule at its enforcement point. The
// only destination-free write gomavlib offers is WriteMessageAll, so a
// fallback here would put a command addressed to system 2 on system 1's radio
// and multiply uplink bandwidth by the number of links.
var ErrNoRoute = errors.New("routes: no live route to target")

// Key identifies a routable MAVLink node.
//
// Component ID is part of the key, not dropped. A vehicle is several
// components — autopilot, gimbal, companion computer — and they can be reached
// on different links; keying on system ID alone silently addresses whichever
// component spoke last.
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
	// Link is the routable address: the channel gomavlib owns, named by the
	// codec's LinkID. The Tier 4 plan stored a *gomavlib.Channel here; the
	// codec already holds the channel and exposes WriteTo(LinkID, msg), so
	// storing the ID keeps this package free of gomavlib and leaves exactly
	// one owner of the socket. The distinction that mattered is preserved: the
	// address is the link, never the IP.
	Link codec.LinkID
	// SrcIP and SrcPort are diagnostics only — they answer "where did this
	// come from" for an operator, and are never used to send. On a UDP server
	// endpoint the peer address is what gomavlib replies to; duplicating it
	// here would create a second, staler answer.
	SrcIP   string
	SrcPort int

	LastSeenMs int64

	Key Key
}

// Table is the concurrent route table.
//
// It is read by the command send path and written by the transport receive
// path, so every access is guarded. There is no background goroutine: nothing
// here needs to happen on its own schedule, and a timer would make the table
// untestable without one.
type Table struct {
	entries map[Key]Entry
	mu      sync.RWMutex
}

// NewTable returns an empty table.
func NewTable() *Table {
	return &Table{entries: make(map[Key]Entry)}
}

// Upsert records that a vehicle was just heard on a link.
//
// A vehicle appearing on a new link moves rather than being duplicated: the
// table answers "where do I send to reach this node now", and that has exactly
// one answer. SrcIP and SrcPort are carried forward from the existing entry
// when the caller supplies none, so a transport that cannot attribute a peer
// address does not erase one we already had.
func (t *Table) Upsert(e Entry, nowMs int64) {
	e.LastSeenMs = nowMs

	t.mu.Lock()
	defer t.mu.Unlock()

	if prev, ok := t.entries[e.Key]; ok && e.SrcIP == "" {
		e.SrcIP, e.SrcPort = prev.SrcIP, prev.SrcPort
	}

	t.entries[e.Key] = e
}

// Lookup returns the live route for a target.
//
// Eviction is lazy and scoped to the key being looked at: a stale entry is
// removed and reported as absent. The scan of every other entry is Evict's
// job. Folding a full sweep into a point lookup would make the latency of a
// single command depend on fleet size, and the sweep has a caller already.
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

// Resolve returns the link to address a target on, or ErrNoRoute.
//
// This is the send path's entry point. It returns an error rather than a
// zero LinkID so that "no route" cannot be mistaken for a usable destination
// by a caller that forgot to check a boolean.
func (t *Table) Resolve(key Key, nowMs int64) (codec.LinkID, error) {
	e, ok := t.Lookup(key, nowMs)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoRoute, key)
	}

	return e.Link, nil
}

// Evict removes every entry older than the route TTL and returns how many went.
//
// Called from the same sweep that expires vehicles, not from a timer of its
// own — one clock for liveness, one place it advances.
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

// ForgetLink drops every route pointing at a link and returns how many went.
//
// Called when a channel closes. A route through a dead channel is worse than
// no route: it makes an unreachable vehicle look addressable, and the command
// fails at the socket instead of at the decision.
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
