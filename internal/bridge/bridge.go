// Package bridge joins the codec's frame stream to the vehicle fold and
// forwards what the fold produces to a sink.
//
// This is the first package in the repo that owns a goroutine. Everything
// below it is pure: the codec turns bytes into envelopes, the fold turns an
// envelope plus a timestamp into new state plus events, and the route table
// answers where to send. None of them decide *when*. This package does, and it
// is deliberately the only one that does.
//
// # One loop, not one goroutine per vehicle
//
// The Tier 5 plan called for a goroutine per discovered vehicle, spawned on
// first HEARTBEAT under a sync.Once guard and cancelled on VEHICLE_LOST. That
// shape is not used, for three reasons found by reading Tier 4 as built:
//
//   - The fold is pure and cheap. A single loop holding map[routes.Key]State
//     and calling Fold sequentially has the same semantics with no lifetime
//     problem, no per-vehicle context to cancel and no leak surface.
//   - VEHICLE_RECOVERED did not exist when the plan was written. A vehicle
//     that is lost and comes back needs its goroutine again, which is exactly
//     what a sync.Once keyed on system ID prevents.
//   - Fold order across vehicles becomes nondeterministic the moment the
//     folds run concurrently, and the replayability the fold was built for is
//     the thing that makes this system testable.
//
// The loop is single-owner: `states` is touched from nowhere else and needs no
// mutex. The route table does have a mutex because the Tier 8 send path reads
// it from another goroutine.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gomavlib/v3"

	"yalb.gcs/internal/codec"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

// SweepInterval is how often the loop asks the fold about vehicles that have
// sent nothing.
//
// Fold only runs when a frame arrives, so a vehicle that goes silent for good
// would never be declared lost without this tick. One second rather than
// something proportional to codec.HeartbeatTTL: the sweep is a map walk over a
// handful of vehicles, and a coarser tick only adds latency between the TTL
// actually expiring and the operator being told.
const SweepInterval = time.Second

// ErrNoSource is returned by New when no frame source was configured.
var ErrNoSource = errors.New("bridge: Config.Source is required")

// Config assembles a Bridge. Only Source is required.
type Config struct {
	// Source is the transport port — codec.Node in production, anything
	// implementing the interface in tests.
	Source codec.FrameSource
	// Sink receives every event the fold emits. Defaults to NopSink.
	Sink Sink
	// Routes is the send path's address book, populated from inbound frames.
	// Defaults to a fresh table, readable afterwards via Bridge.Routes.
	Routes *routes.Table
	// Logger receives link lifecycle lines. Defaults to slog.Default().
	Logger *slog.Logger
	// Now returns the current time in epoch milliseconds. Injected because
	// every property worth testing here — discovery, loss, recovery, route
	// eviction — is a statement about time. Defaults to the wall clock, which
	// is the one place in the receive path allowed to read it.
	Now func() int64
	// Sweep overrides SweepInterval. Tests shorten it.
	Sweep time.Duration
}

// Bridge is the running receive pipeline.
type Bridge struct {
	source codec.FrameSource
	sink   Sink
	routes *routes.Table
	log    *slog.Logger
	now    func() int64
	states map[routes.Key]vehicle.State
	sweep  time.Duration
}

// New validates and defaults a Config into a Bridge. It starts nothing.
func New(cfg Config) (*Bridge, error) {
	if cfg.Source == nil {
		return nil, ErrNoSource
	}

	b := &Bridge{
		source: cfg.Source,
		sink:   cfg.Sink,
		routes: cfg.Routes,
		log:    cfg.Logger,
		now:    cfg.Now,
		states: make(map[routes.Key]vehicle.State),
		sweep:  cfg.Sweep,
	}

	if b.sink == nil {
		b.sink = NopSink{}
	}

	if b.routes == nil {
		b.routes = routes.NewTable()
	}

	if b.log == nil {
		b.log = slog.Default()
	}

	if b.now == nil {
		b.now = func() int64 { return time.Now().UnixMilli() }
	}

	if b.sweep <= 0 {
		b.sweep = SweepInterval
	}

	return b, nil
}

// Routes exposes the route table so the send path can resolve targets.
func (b *Bridge) Routes() *routes.Table { return b.routes }

// Run drives the pipeline until the context is cancelled or the source closes.
//
// It returns nil on both of those, because both are the caller shutting things
// down rather than a failure. A non-nil return means the sink refused an
// event, which is a real fault: the fold produced an observation nothing
// recorded.
func (b *Bridge) Run(ctx context.Context) error {
	ticker := time.NewTicker(b.sweep)
	defer ticker.Stop()

	events := b.source.Events()

	for {
		select {
		case <-ctx.Done():
			b.log.InfoContext(ctx, "bridge stopping", "reason", "context cancelled")

			return nil

		case evt, ok := <-events:
			if !ok {
				b.log.InfoContext(ctx, "bridge stopping", "reason", "frame source closed")

				return nil
			}

			if err := b.handle(ctx, evt); err != nil {
				return err
			}

		case <-ticker.C:
			if err := b.sweepOnce(ctx); err != nil {
				return err
			}
		}
	}
}

// handle dispatches one gomavlib event.
//
// Unrecognised event types are ignored rather than defaulted into an error:
// gomavlib adds events (EventStreamRequested, and whatever comes next) and a
// new one is not a fault in this bridge.
func (b *Bridge) handle(ctx context.Context, evt gomavlib.Event) error {
	switch e := evt.(type) {
	case *gomavlib.EventChannelOpen:
		b.log.InfoContext(ctx, "link open", "link", e.Channel.String())

	case *gomavlib.EventChannelClose:
		// The codec has already dropped its own link and route entries; this
		// drops ours. A route pointing at a closed channel would make a
		// vehicle look addressable, and the write would fail at the far end of
		// the send path instead of being rejected here.
		link := codec.LinkID(e.Channel.String())
		b.log.InfoContext(ctx, "link closed",
			"link", link,
			"err", e.Error,
			"routes_dropped", b.routes.ForgetLink(link),
		)

	case *gomavlib.EventParseError:
		// codec.Node counts these; this line names one. Debug because a noisy
		// radio produces a stream of them and the count is the signal, not any
		// individual failure.
		b.log.DebugContext(ctx, "parse error", "link", e.Channel.String(), "err", e.Error)

	case *gomavlib.EventFrame:
		return b.frame(ctx, e)
	}

	return nil
}

// frame folds one decoded frame and publishes what it produced.
func (b *Bridge) frame(ctx context.Context, e *gomavlib.EventFrame) error {
	// Frames carrying our own identity are dropped before anything else sees
	// them. gomavlib does not loop our writes back to Events(), so in a
	// healthy Compose topology this never fires — but a misconfigured UDP
	// route, a mavproxy in the path, or a second GCS on the network all
	// deliver (255, 190) frames, and folding those creates a phantom "vehicle
	// 255" in fleet:active that no amount of downstream filtering can undo.
	if e.SystemID() == codec.GCSSystemID && e.ComponentID() == codec.GCSComponentID {
		return nil
	}

	now := b.now()
	link := codec.LinkID(e.Channel.String())
	key := routes.Key{SysID: e.SystemID(), CompID: e.ComponentID()}

	srcIP, srcPort := splitLabel(e.Channel.String())
	b.routes.Upsert(routes.Entry{
		Key:     key,
		Link:    link,
		SrcIP:   srcIP,
		SrcPort: srcPort,
	}, now)

	// SrcAddr is the channel label, not a parsed IP. gomavlib does not put a
	// peer address on EventFrame, and for a UDP server endpoint it does not
	// need to: it opens one channel per remote peer and labels it
	// "udp:<host>:<port>", so the label already identifies the source at
	// exactly the granularity the conflict check wants. Using it also keeps
	// the check meaningful on a serial link, where there is no IP at all.
	in := vehicle.Inbound{
		Heartbeat: codec.DecodeHeartbeat(e),
		SrcAddr:   e.Channel.String(),
		Decoded:   codec.Decode(e),
		MsgID:     e.Message().GetID(),
	}

	next, events := vehicle.Fold(b.states[key], in, now)
	b.states[key] = next

	return b.emit(ctx, events)
}

// sweepOnce advances every known vehicle against the clock and evicts stale
// routes.
func (b *Bridge) sweepOnce(ctx context.Context) error {
	now := b.now()

	for key, state := range b.states {
		next, events := vehicle.Expire(state, now)
		b.states[key] = next

		if err := b.emit(ctx, events); err != nil {
			return err
		}
	}

	if n := b.routes.Evict(now); n > 0 {
		b.log.InfoContext(ctx, "routes evicted", "count", n)
	}

	return nil
}

// emit publishes a fold's output in order.
func (b *Bridge) emit(ctx context.Context, events []vehicle.Event) error {
	for _, ev := range events {
		if err := b.sink.Publish(ctx, ev); err != nil {
			return fmt.Errorf("bridge: publishing event: %w", err)
		}
	}

	return nil
}

// splitLabel recovers a host and port from a gomavlib channel label.
//
// Diagnostics only — the routable address is the LinkID, never this. Labels
// are "<endpoint kind>:<remote address>" for server endpoints and a bare name
// for client and custom ones, so a label with no address yields zero values
// and routes.Upsert carries forward whatever it already had.
func splitLabel(label string) (host string, port int) {
	_, addr, ok := strings.Cut(label, ":")
	if !ok {
		return "", 0
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0
	}

	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", 0
	}

	return host, port
}
