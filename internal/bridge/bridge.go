// Package bridge runs the single-owner receive loop from frames through the
// vehicle fold to an event sink.
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

// SweepInterval bounds loss-detection and stale-route latency.
const SweepInterval = time.Second

// ErrNoSource is returned by New when no frame source was configured.
var ErrNoSource = errors.New("bridge: Config.Source is required")

// Config assembles a Bridge. Only Source is required.
type Config struct {
	// Source is the transport port.
	Source codec.FrameSource
	// Sink receives every event the fold emits. Defaults to NopSink.
	Sink Sink
	// Routes is populated from inbound frames.
	Routes *routes.Table
	// Logger receives link lifecycle lines. Defaults to slog.Default().
	Logger *slog.Logger
	// Now returns epoch milliseconds and is injectable for deterministic tests.
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

// Run drives the pipeline. Cancellation and source closure are clean exits;
// sink failures are returned.
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

// handle dispatches one recognized gomavlib event.
func (b *Bridge) handle(ctx context.Context, evt gomavlib.Event) error {
	switch e := evt.(type) {
	case *gomavlib.EventChannelOpen:
		b.log.InfoContext(ctx, "link open", "link", e.Channel.String())

	case *gomavlib.EventChannelClose:
		link := codec.LinkID(e.Channel.String())
		b.log.InfoContext(ctx, "link closed",
			"link", link,
			"err", e.Error,
			"routes_dropped", b.routes.ForgetLink(link),
		)

	case *gomavlib.EventParseError:
		b.log.DebugContext(ctx, "parse error", "link", e.Channel.String(), "err", e.Error)

	case *gomavlib.EventFrame:
		return b.frame(ctx, e)
	}

	return nil
}

// frame folds one decoded frame and publishes what it produced.
func (b *Bridge) frame(ctx context.Context, e *gomavlib.EventFrame) error {
	// Ignore externally relayed frames carrying this GCS's identity.
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

	// Channel labels identify sources across UDP, serial, and custom links.
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

// splitLabel extracts optional diagnostic host and port values.
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
