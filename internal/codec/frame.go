// Package codec wraps MAVLink framing and dialect handling.
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

// GCS node identity used for outbound MAVLink frames.
const (
	GCSSystemID    byte = 255
	GCSComponentID byte = 190
)

// HeartbeatPeriod sets gomavlib's per-link GCS heartbeat rate.
const HeartbeatPeriod = time.Second

// HeartbeatTTL is the vehicle liveness timeout.
const HeartbeatTTL = 60 * time.Second

// LinkIdleTimeout exceeds HeartbeatTTL so vehicle loss precedes link teardown.
const LinkIdleTimeout = 3 * HeartbeatTTL

// LinkID identifies one open MAVLink channel.
type LinkID string

// ErrUnknownLink is returned when an addressed write has no open link.
var ErrUnknownLink = errors.New("codec: no open link for target")

// FrameSource is the transport port. Implementations deliver decoded gomavlib
// events and accept addressed writes.
type FrameSource interface {
	Events() <-chan gomavlib.Event
	WriteTo(link LinkID, msg message.Message) error
	Close() error
}

var _ FrameSource = (*Node)(nil)

// Node is the gomavlib-backed FrameSource and inbound-derived route map.
type Node struct {
	node    *gomavlib.Node
	events  chan gomavlib.Event
	closing chan struct{}

	// links and routes are guarded by mu.
	links  map[LinkID]*gomavlib.Channel
	routes map[byte]LinkID
	mu     sync.RWMutex

	// parseErrors counts frames rejected before decode.
	parseErrors atomic.Uint64

	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewNode starts a gomavlib node over the supplied endpoints.
func NewNode(endpoints []gomavlib.EndpointConf) (*Node, error) {
	inner := &gomavlib.Node{
		Endpoints:      endpoints,
		Dialect:        ardupilotmega.Dialect,
		OutVersion:     gomavlib.V2,
		OutSystemID:    GCSSystemID,
		OutComponentID: GCSComponentID,

		HeartbeatDisable: false,
		HeartbeatPeriod:  HeartbeatPeriod,

		IdleTimeout: LinkIdleTimeout,

		// REQUEST_DATA_STREAM (#66) is deprecated and offers only coarse stream
		// groups. Keep it disabled: per-message rates are requested above this
		// layer with MAV_CMD_SET_MESSAGE_INTERVAL, on discovery and recovery,
		// so gomavlib must not also be negotiating streams of its own.
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

// Events returns decoded events after their routes have been observed.
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
func (n *Node) LinkFor(sysID byte) (LinkID, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	link, ok := n.routes[sysID]

	return link, ok
}

// ParseErrors returns the number of frames rejected before decode.
func (n *Node) ParseErrors() uint64 {
	return n.parseErrors.Load()
}

// Close halts the node and waits for its goroutines.
func (n *Node) Close() error {
	n.closeOnce.Do(func() {
		// Release a blocked event send before gomavlib waits on its producers.
		close(n.closing)
		n.node.Close()
		n.wg.Wait()
	})

	return nil
}

// pump forwards events while maintaining links and routes. Closing interrupts
// a blocked consumer send; in-flight events may be dropped during shutdown.
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
		n.parseErrors.Add(1)

	case *gomavlib.EventFrame:
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
