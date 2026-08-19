package bridge

import (
	"context"
	"log/slog"

	"yalb.gcs/internal/vehicle"
)

// Sink is where the fold's output goes.
//
// It exists so the router loop can be finished, tested and run against live
// SITL before Redis is wired. The Tier 5 plan assembled the pipeline straight
// into a Redis publisher, which makes the first runnable version of the bridge
// depend on a running Redis — and makes every failure ambiguous between "the
// fold is wrong" and "the publisher is wrong". One interface separates them.
//
// Publish is called from the single router goroutine, in fold order, one event
// at a time. An implementation that blocks stalls the receive path, so a slow
// sink must buffer internally rather than doing its I/O inline.
type Sink interface {
	Publish(ctx context.Context, ev vehicle.Event) error
}

// NopSink discards everything. For tests that assert on state rather than
// output, and for a bridge run with no downstream configured.
type NopSink struct{}

var _ Sink = NopSink{}

// Publish discards the event.
func (NopSink) Publish(context.Context, vehicle.Event) error { return nil }

// LogSink writes the fold's output to a structured logger.
//
// This is the Tier 5 first-light sink: `docker compose up` and read the log to
// see whether a vehicle was discovered. Telemetry logs at Debug deliberately —
// a fleet of three SITL instances produces a few hundred telemetry events per
// second, and logging those at Info makes the fleet events they are meant to
// contextualise unreadable.
type LogSink struct {
	Log *slog.Logger
}

var _ Sink = LogSink{}

// Publish renders one event as a log line.
func (s LogSink) Publish(ctx context.Context, ev vehicle.Event) error {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}

	switch {
	case ev.Fleet != nil:
		log.InfoContext(ctx, "fleet event",
			"type", ev.Fleet.GetType().String(),
			"sysid", ev.Fleet.GetVehicleId().GetSystemId(),
			"compid", ev.Fleet.GetVehicleId().GetComponentId(),
			"mav_type", ev.Fleet.GetHeartbeat().GetType().String(),
			"armed", ev.Fleet.GetHeartbeat().GetArmed(),
		)

	case ev.Warning != nil:
		// Warn, not Info. A source conflict means either a duplicated system
		// ID or injected frames; see the transport threat model in
		// docs/roadmap/tier-4-bridge-core.md.
		log.WarnContext(ctx, "vehicle warning", "warning", ev.Warning.String())

	case ev.Telemetry != nil:
		log.DebugContext(ctx, "telemetry",
			"sysid", ev.Telemetry.GetVehicleId().GetSystemId(),
		)
	}

	return nil
}
