package bridge

import (
	"context"
	"log/slog"

	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/vehicle"
)

// Sink receives fold output in order. Slow implementations must buffer.
type Sink interface {
	Publish(ctx context.Context, ev vehicle.Event) error
}

// MultiSink fans one event out to several sinks, in declaration order.
//
// Order is the contract: the sinks see the same events in the same sequence
// the fold produced them, so a rate request and the log line describing it
// cannot disagree about what happened first. The first failure stops the fan-
// out and stops the bridge, for the reason Sink failures always do — a receive
// path that keeps running while a consumer is dropping observations reports
// health it does not have.
type MultiSink []Sink

var _ Sink = MultiSink(nil)

// Publish forwards to each sink until one fails.
func (m MultiSink) Publish(ctx context.Context, ev vehicle.Event) error {
	for _, s := range m {
		if err := s.Publish(ctx, ev); err != nil {
			return err
		}
	}

	return nil
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

	case ev.Protocol != nil:
		s.protocol(ctx, log, ev.Protocol)
	}

	return nil
}

// protocol renders a transaction response. COMMAND_ACK is logged at Info
// because it is the only feedback the rate requests get: without it, an
// autopilot refusing a message interval is indistinguishable from one that was
// never asked.
func (s LogSink) protocol(ctx context.Context, log *slog.Logger, ev *gcsv1.ProtocolEvent) {
	ack := ev.GetCommandAck()
	if ack == nil {
		log.DebugContext(ctx, "protocol event",
			"sysid", ev.GetVehicleId().GetSystemId(),
			"compid", ev.GetVehicleId().GetComponentId(),
		)

		return
	}

	log.InfoContext(ctx, "command ack",
		"sysid", ev.GetVehicleId().GetSystemId(),
		"compid", ev.GetVehicleId().GetComponentId(),
		"command", ack.GetCommand(),
		"result", ack.GetResult().String(),
	)
}
