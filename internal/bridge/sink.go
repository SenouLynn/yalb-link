package bridge

import (
	"context"
	"log/slog"

	"yalb.gcs/internal/vehicle"
)

// Sink receives fold output in order. Slow implementations must buffer.
type Sink interface {
	Publish(ctx context.Context, ev vehicle.Event) error
}

// NopSink discards all events.
type NopSink struct{}

var _ Sink = NopSink{}

// Publish discards the event.
func (NopSink) Publish(context.Context, vehicle.Event) error { return nil }

// LogSink writes fold output to a structured logger; telemetry uses Debug.
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
		log.WarnContext(ctx, "vehicle warning", "warning", ev.Warning.String())

	case ev.Telemetry != nil:
		log.DebugContext(ctx, "telemetry",
			"sysid", ev.Telemetry.GetVehicleId().GetSystemId(),
		)
	}

	return nil
}
