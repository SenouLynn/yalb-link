// Package stream publishes bridge events to browser subscribers over SSE.
//
// The hub is deliberately in-memory and lossy at the edge: it retains the
// latest state per vehicle so a browser that connects late still sees a
// complete picture, and it drops a subscriber that cannot keep up rather than
// letting one slow tab apply back-pressure to the MAVLink receive loop.
package stream

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"google.golang.org/protobuf/proto"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

// SSE event names. These are the whole public event vocabulary of /api/events.
const (
	// EventFleet carries a FleetEvent: discovery, loss, recovery, heartbeat.
	EventFleet = "fleet"
	// EventTelemetry carries a TelemetryEvent: one streaming payload.
	EventTelemetry = "telemetry"
	// EventCommand carries an operator command transaction snapshot.
	EventCommand = "command"
)

// DefaultQueue is the live headroom each subscriber gets beyond its bootstrap.
//
// Sized for a burst, not a backlog. A subscriber this far behind is not going
// to catch up on a stream that keeps producing, and holding more would only
// delay the disconnect while showing the operator staler data.
const DefaultQueue = 256

// Event is one thing to send to a browser: an SSE event name and the protobuf
// message that becomes its data field.
type Event struct {
	Message proto.Message
	Name    string
}

// vehicleKey identifies a vehicle by its full MAVLink identity. Component ID is
// part of the key because a gimbal and its autopilot share a system ID and are
// not the same vehicle.
type vehicleKey struct {
	sysID  uint32
	compID uint32
}

func (k vehicleKey) compare(other vehicleKey) int {
	if c := cmp.Compare(k.sysID, other.sysID); c != 0 {
		return c
	}

	return cmp.Compare(k.compID, other.compID)
}

// Hub retains the latest fleet and telemetry state and fans live events out to
// subscribers.
type Hub struct {
	fleet     map[vehicleKey]*gcsv1.FleetEvent
	telemetry map[vehicleKey]map[string]*gcsv1.TelemetryEvent
	subs      map[*subscriber]struct{}
	log       *slog.Logger
	queue     int
	mu        sync.Mutex
}

// NewHub returns an empty hub. queue bounds each subscriber's live backlog;
// zero selects DefaultQueue.
func NewHub(log *slog.Logger, queue int) *Hub {
	if log == nil {
		log = slog.Default()
	}

	if queue <= 0 {
		queue = DefaultQueue
	}

	return &Hub{
		fleet:     make(map[vehicleKey]*gcsv1.FleetEvent),
		telemetry: make(map[vehicleKey]map[string]*gcsv1.TelemetryEvent),
		subs:      make(map[*subscriber]struct{}),
		log:       log,
		queue:     queue,
	}
}

// subscriber is one connected browser.
type subscriber struct {
	events chan Event
	once   sync.Once
}

// close releases the subscriber's channel exactly once. Always called with the
// hub lock held, so it cannot race a send.
func (s *subscriber) close() {
	s.once.Do(func() { close(s.events) })
}

// Subscribe registers a subscriber and returns its event channel already
// primed with the current state, plus the function that unregisters it.
//
// Registration and bootstrap happen under one lock. If they did not, an event
// published in between would be missed by a subscriber that had not yet been
// registered but whose snapshot had already been taken — the classic gap that
// leaves a browser showing a vehicle that has since disappeared.
//
// The channel is sized to hold the entire bootstrap plus the live headroom, so
// priming it can never block and can never evict the subscriber being created.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	bootstrap := h.snapshot()

	sub := &subscriber{events: make(chan Event, len(bootstrap)+h.queue)}
	for _, ev := range bootstrap {
		sub.events <- ev
	}

	h.subs[sub] = struct{}{}

	return sub.events, func() { h.unsubscribe(sub) }
}

// unsubscribe drops a subscriber. Safe to call more than once.
func (h *Hub) unsubscribe(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subs, sub)
	sub.close()
}

// Subscribers reports how many subscribers are connected, for diagnostics.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.subs)
}

// snapshot renders the retained state as bootstrap events. Fleet state comes
// first so a subscriber knows which vehicles exist before it is told anything
// about them, and both halves are ordered by identity so two browsers
// connecting to the same hub receive byte-identical bootstraps.
//
// Callers must hold h.mu.
func (h *Hub) snapshot() []Event {
	keys := make([]vehicleKey, 0, len(h.fleet))
	for key := range h.fleet {
		keys = append(keys, key)
	}

	// Telemetry can exist for a vehicle that has not produced a fleet event, so
	// the ordering walk covers both maps.
	for key := range h.telemetry {
		if _, ok := h.fleet[key]; !ok {
			keys = append(keys, key)
		}
	}

	slices.SortFunc(keys, vehicleKey.compare)

	out := make([]Event, 0, len(keys))

	for _, key := range keys {
		if evt, ok := h.fleet[key]; ok {
			out = append(out, Event{Name: EventFleet, Message: evt})
		}
	}

	for _, key := range keys {
		families := slices.Sorted(maps.Keys(h.telemetry[key]))

		for _, family := range families {
			out = append(out, Event{Name: EventTelemetry, Message: h.telemetry[key][family]})
		}
	}

	return out
}

// Publish records an event and fans it out. It implements bridge.Sink.
//
// It never returns an error. Sink failures stop the bridge, and no browser is
// entitled to do that: the receive path's job is to observe the vehicle, and it
// must keep doing that with zero subscribers, one stalled subscriber, or a
// hundred.
func (h *Hub) Publish(ctx context.Context, ev vehicle.Event) error {
	switch {
	case ev.Fleet != nil:
		h.record(ctx, keyOfFleet(ev.Fleet), Event{Name: EventFleet, Message: ev.Fleet})

	case ev.Telemetry != nil:
		h.record(ctx, keyOfTelemetry(ev.Telemetry), Event{Name: EventTelemetry, Message: ev.Telemetry})

	case ev.Command != nil:
		// Transactions are history, not retained current state.
		h.broadcast(ctx, Event{Name: EventCommand, Message: ev.Command})
	}

	// Warnings and protocol events are logged, not streamed.
	return nil
}

func (h *Hub) broadcast(ctx context.Context, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deliver(ctx, ev)
}

// record retains an event and delivers it to every subscriber that has room.
func (h *Hub) record(ctx context.Context, key vehicleKey, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch msg := ev.Message.(type) {
	case *gcsv1.FleetEvent:
		h.fleet[key] = msg

	case *gcsv1.TelemetryEvent:
		families, ok := h.telemetry[key]
		if !ok {
			families = make(map[string]*gcsv1.TelemetryEvent)
			h.telemetry[key] = families
		}

		families[familyOf(msg)] = msg
	}

	h.deliver(ctx, ev)
}

// deliver sends a live event. Callers hold h.mu.
func (h *Hub) deliver(ctx context.Context, ev Event) {
	for sub := range h.subs {
		select {
		case sub.events <- ev:
		default:
			// The subscriber is beyond its backlog. Disconnect it; the browser
			// reconnects and gets a fresh, correct bootstrap rather than
			// resuming a stream it has already fallen out of sync with.
			delete(h.subs, sub)
			sub.close()

			h.log.WarnContext(ctx, "subscriber evicted",
				"reason", "queue full",
				"queue", cap(sub.events),
			)
		}
	}
}

// Close disconnects every subscriber. Retained state is left intact so a hub
// can be closed and resubscribed to in tests.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subs {
		delete(h.subs, sub)
		sub.close()
	}
}

func keyOfFleet(ev *gcsv1.FleetEvent) vehicleKey {
	return vehicleKey{
		sysID:  ev.GetVehicleId().GetSystemId(),
		compID: ev.GetVehicleId().GetComponentId(),
	}
}

func keyOfTelemetry(ev *gcsv1.TelemetryEvent) vehicleKey {
	return vehicleKey{
		sysID:  ev.GetVehicleId().GetSystemId(),
		compID: ev.GetVehicleId().GetComponentId(),
	}
}

// familyOf names a telemetry event's payload by its oneof field name, so
// retention is per message family. Read from the descriptor rather than a type
// switch: a family this does not know about would otherwise share a retention
// slot with another and silently overwrite it.
func familyOf(ev *gcsv1.TelemetryEvent) string {
	msg := ev.ProtoReflect()

	field := msg.WhichOneof(msg.Descriptor().Oneofs().ByName("payload"))
	if field == nil {
		return ""
	}

	return string(field.Name())
}
