// Package codec wraps gomavlib's framing and dialect handling behind ports the
// rest of the GCS depends on.
//
// This package imports nothing above itself: no Redis, no Connect services, no
// vehicle model. Everything here is driven by bytes in and typed messages out,
// so the decode path is testable over a net.Pipe with no UDP socket open.
package codec

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/message"
)

// GCS node identity. Fixed by the heartbeat-fields decision in
// docs/roadmap/order-of-operations.md: a GCS is (255, 190).
const (
	GCSSystemID    byte = 255
	GCSComponentID byte = 190
)

// HeartbeatPeriod overrides gomavlib's 5s default.
//
// gomavlib already emits a conforming GCS heartbeat on every open channel:
// HeartbeatDisable defaults to false, HeartbeatSystemType to MAV_TYPE_GCS(6)
// and HeartbeatAutopilotType to MAV_AUTOPILOT_GENERIC(0). Only the rate is
// wrong. Do not add a second hand-rolled ticker — heartbeat is a per-link
// broadcast, not a per-vehicle message, and a per-vehicle ticker both
// double-emits and scales with fleet size on the scarce uplink direction.
const HeartbeatPeriod = time.Second

// LinkID identifies one open channel — one radio, one UDP peer, one SITL
// instance. Writes are addressed to a link, never broadcast.
type LinkID string

// ErrUnknownLink is returned by WriteTo when the target link is not open.
//
// This is the "reject, never broadcast" rule. gomavlib offers WriteMessageAll
// as the only destination-free write, and falling back to it would put a
// command for sysid 2 on vehicle 1's radio and multiply uplink bandwidth by
// the number of links. An unaddressable target is an error.
var ErrUnknownLink = errors.New("codec: no open link for target")

// FrameSource is the transport port. Implementations deliver decoded gomavlib
// events and accept addressed writes.
type FrameSource interface {
	Events() <-chan gomavlib.Event
	WriteTo(link LinkID, msg message.Message) error
	Close() error
}

var _ FrameSource = (*Node)(nil)

// Node is the gomavlib-backed FrameSource.
//
// Beyond wrapping the node it maintains the routing table: which link each
// vehicle system ID was last heard on. The table is populated from inbound
// frames only — nothing pre-registers a vehicle, and a vehicle that has never
// been heard from is not addressable.
type Node struct {
	// Field order is packed pointer-like-first to satisfy govet's
	// fieldalignment, then grouped by ownership. The mu/links/routes grouping
	// is the one that matters for readers: mu guards the two maps above it.
	node    *gomavlib.Node
	events  chan gomavlib.Event
	closing chan struct{}

	// links and routes are guarded by mu.
	links  map[LinkID]*gomavlib.Channel
	routes map[byte]LinkID
	mu     sync.RWMutex

	// parseErrors counts frames gomavlib rejected: bad CRC, truncated, or a
	// signing failure.
	//
	// Dropping an unrecognised message ID silently is correct — we do not
	// handle every message in the dialect and never will. Dropping a *parse
	// error* silently is not: it is the signal for a failing radio, an
	// electrically noisy airframe, or replay tampering, and it is invisible
	// unless something counts it.
	parseErrors atomic.Uint64

	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewNode configures and starts a gomavlib node over the given endpoints.
//
// Endpoints are injected by the caller so tests can supply
// gomavlib.EndpointCustomClient over a net.Pipe and production can supply a
// UDP server, without this package knowing the difference.
func NewNode(endpoints []gomavlib.EndpointConf) (*Node, error) {
	// gomavlib.NodeConf and gomavlib.NewNode are both deprecated in v3
	// ("configuration has been moved inside Node") and staticcheck SA1019 is
	// enabled in .golangci.yml, so the deprecated path fails our own lint gate.
	// Configure the struct and call Initialize.
	inner := &gomavlib.Node{
		Endpoints:      endpoints,
		Dialect:        ardupilotmega.Dialect,
		OutVersion:     gomavlib.V2,
		OutSystemID:    GCSSystemID,
		OutComponentID: GCSComponentID,

		HeartbeatDisable: false,
		HeartbeatPeriod:  HeartbeatPeriod,

		// StreamRequestEnable stays false deliberately. SITL over UDP streams
		// telemetry unprompted, so leaving this on would make Tiers 5-6 look
		// healthy while masking that a real ArduPilot link may deliver almost
		// nothing until SET_MESSAGE_INTERVAL is sent (Tier 8a).
		StreamRequestEnable: false,
	}

	if err := inner.Initialize(); err != nil {
		return nil, fmt.Errorf("codec: initializing gomavlib node: %w", err)
	}

	n := &Node{
		node:    inner,
		events:  make(chan gomavlib.Event),
		links:   make(map[LinkID]*gomavlib.Channel),
		routes:  make(map[byte]LinkID),
		closing: make(chan struct{}),
	}

	n.wg.Add(1)
	go n.pump()

	return n, nil
}

// Events returns the stream of decoded events.
//
// Events are forwarded from the underlying node after the routing table has
// been updated, so a consumer that reacts to a frame can always address a
// reply back to its source link.
func (n *Node) Events() <-chan gomavlib.Event {
	return n.events
}

// WriteTo writes a message to one link. It never falls back to a broadcast.
func (n *Node) WriteTo(link LinkID, msg message.Message) error {
	n.mu.RLock()
	ch, ok := n.links[link]
	n.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownLink, link)
	}

	if err := n.node.WriteMessageTo(ch, msg); err != nil {
		return fmt.Errorf("codec: writing to link %q: %w", link, err)
	}

	return nil
}

// LinkFor reports the link a vehicle system ID was last heard on.
//
// Callers addressing a vehicle resolve through here and propagate the
// not-found case as a rejection.
func (n *Node) LinkFor(sysID byte) (LinkID, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	link, ok := n.routes[sysID]

	return link, ok
}

// ParseErrors returns the number of frames gomavlib rejected as unparseable.
//
// A rising count on an otherwise healthy link points at the physical layer,
// not at this codec.
func (n *Node) ParseErrors() uint64 {
	return n.parseErrors.Load()
}

// Close halts the node and waits for its goroutines to return.
//
// Node.Initialize starts three goroutines, so the claim under goleak is "none
// leaked", not "none running".
func (n *Node) Close() error {
	n.closeOnce.Do(func() {
		// Order matters. Releasing a pump that is blocked mid-send has to come
		// first: gomavlib's Close waits for its own goroutines, and one of them
		// can be blocked writing the event that pump is blocked forwarding.
		// Closing the node first in that state deadlocks both.
		close(n.closing)
		n.node.Close()
		n.wg.Wait()
	})

	return nil
}

// pump forwards node events, maintaining the link and route tables as it goes.
//
// The send is guarded by a select on closing rather than being a bare channel
// send. Events() is unbuffered, so a bare send blocks until a consumer reads —
// and a consumer that has stopped reading (a cancelled subscription, a test
// that took the frame it wanted, a slow downstream) would wedge this goroutine
// forever. Since Close waits on the WaitGroup this goroutine belongs to, that
// turns a stalled consumer into a node that can never be shut down. Selecting
// on closing keeps Close total: it always returns, whatever the consumer does.
//
// Events still in flight when Close is called are dropped. That is deliberate —
// a shutdown path that insists on delivering telemetry nobody is reading is the
// deadlock this exists to prevent.
func (n *Node) pump() {
	defer n.wg.Done()
	defer close(n.events)

	for evt := range n.node.Events() {
		n.observe(evt)

		select {
		case n.events <- evt:
		case <-n.closing:
			return
		}
	}
}

// observe updates the link and route tables from an event.
func (n *Node) observe(evt gomavlib.Event) {
	switch e := evt.(type) {
	case *gomavlib.EventChannelOpen:
		n.mu.Lock()
		n.links[linkIDOf(e.Channel)] = e.Channel
		n.mu.Unlock()

	case *gomavlib.EventChannelClose:
		n.forgetLink(linkIDOf(e.Channel))

	case *gomavlib.EventParseError:
		// No frame surfaces for these, so this counter is the only trace they
		// leave. Decode never sees them and returns no error for them either.
		n.parseErrors.Add(1)

	case *gomavlib.EventFrame:
		// The routing table is built from what we hear, and only from what we
		// hear. A vehicle appearing on a new link moves; it is not duplicated.
		link := linkIDOf(e.Channel)

		n.mu.Lock()
		n.routes[e.SystemID()] = link
		n.mu.Unlock()
	}
}

// forgetLink drops a link and every route pointing at it, so a closed radio
// does not leave vehicles addressable through a dead channel.
func (n *Node) forgetLink(link LinkID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.links, link)

	for sysID, routed := range n.routes {
		if routed == link {
			delete(n.routes, sysID)
		}
	}
}

func linkIDOf(ch *gomavlib.Channel) LinkID {
	return LinkID(ch.String())
}
