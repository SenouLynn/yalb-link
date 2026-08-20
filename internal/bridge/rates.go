package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

// AutopilotComponentID is MAV_COMP_ID_AUTOPILOT1.
//
// Rates are requested from the autopilot only. Gimbals, companion computers,
// and ADS-B receivers share the system ID and answer their own message sets;
// asking each of them for ATTITUDE at 10 Hz is how a shared link gets
// saturated by a GCS that meant to talk to one component.
const AutopilotComponentID uint8 = 1

// RateRequest names one message family and how often it should arrive.
type RateRequest struct {
	// MsgID is the MAVLink message ID being requested.
	MsgID uint32
	// Hz is the requested rate. It is converted to the wire's microsecond
	// interval once, at the point of encoding.
	Hz float64
}

// IntervalUs converts the requested rate to MAV_CMD_SET_MESSAGE_INTERVAL's
// wire unit. A non-positive rate yields the autopilot's own default.
func (r RateRequest) IntervalUs() int32 {
	if r.Hz <= 0 {
		return codec.IntervalDefaultUs
	}

	return int32(float64(time.Second/time.Microsecond) / r.Hz)
}

// DefaultRates is the acquisition policy for the flight display.
//
// These are the families the instrument view reads, at the slowest rate that
// still looks live: attitude drives the artificial horizon and is the only one
// that needs to be smooth, position and speed update the numbers, and the
// health families change slowly enough that 1 Hz is generous. Nothing here is
// negotiated with the vehicle — an autopilot that cannot honour a rate says so
// in its COMMAND_ACK and streams what it can.
var DefaultRates = []RateRequest{
	{MsgID: 30, Hz: 10}, // ATTITUDE
	{MsgID: 33, Hz: 5},  // GLOBAL_POSITION_INT
	{MsgID: 74, Hz: 5},  // VFR_HUD
	{MsgID: 24, Hz: 1},  // GPS_RAW_INT
	{MsgID: 1, Hz: 1},   // SYS_STATUS
	{MsgID: 147, Hz: 1}, // BATTERY_STATUS
	{MsgID: 193, Hz: 1}, // EKF_STATUS_REPORT
}

// RateRequester asks a vehicle for the telemetry the display needs, as soon as
// the fold reports that vehicle is there.
//
// It is a Sink rather than a background loop so that the request is ordered
// against the discovery that triggered it: the route it writes to was recorded
// from the same frame that produced the fleet event.
type RateRequester struct {
	// Source is the transport the requests are written to.
	Source codec.FrameSource
	// Routes resolves a vehicle identity to the link it was heard on.
	Routes *routes.Table
	// Rates is the acquisition policy. Defaults to DefaultRates.
	Rates []RateRequest
	// Log records what was requested. Defaults to slog.Default().
	Log *slog.Logger
	// Now returns epoch milliseconds, matching the bridge's clock.
	Now func() int64
}

var _ Sink = (*RateRequester)(nil)

// Publish requests rates when a vehicle is discovered or recovered.
//
// A write failure is returned, which stops the bridge. A GCS that silently
// failed to ask for telemetry would sit on a healthy-looking link showing
// nothing but heartbeats, and that is the exact failure this milestone exists
// to make impossible.
func (r *RateRequester) Publish(ctx context.Context, ev vehicle.Event) error {
	if !r.triggers(ev.Fleet) {
		return nil
	}

	id := ev.Fleet.GetVehicleId()
	key := routes.Key{
		SysID:  uint8(id.GetSystemId()),
		CompID: uint8(id.GetComponentId()),
	}

	link, err := r.Routes.Resolve(key, r.now())
	if err != nil {
		return fmt.Errorf("bridge: routing rate requests to %s: %w", key, err)
	}

	target := codec.Target{SystemID: key.SysID, ComponentID: key.CompID}

	for _, req := range r.rates() {
		msg := codec.EncodeSetMessageInterval(target, req.MsgID, req.IntervalUs())

		if err := r.Source.WriteTo(link, msg); err != nil {
			return fmt.Errorf("bridge: requesting message %d from %s: %w", req.MsgID, key, err)
		}
	}

	r.log().InfoContext(ctx, "telemetry rates requested",
		"sysid", key.SysID,
		"compid", key.CompID,
		"link", link,
		"families", len(r.rates()),
		"trigger", ev.Fleet.GetType().String(),
	)

	return nil
}

// triggers reports whether this event means a vehicle just became reachable.
//
// HEARTBEAT_UPDATED is deliberately excluded: it fires on every arm, mode
// change, and system-state transition, and re-requesting the whole policy each
// time would put a burst of commands on the link exactly when the operator is
// doing something.
func (r *RateRequester) triggers(fleet *gcsv1.FleetEvent) bool {
	if fleet == nil {
		return false
	}

	if fleet.GetVehicleId().GetComponentId() != uint32(AutopilotComponentID) {
		return false
	}

	switch fleet.GetType() {
	case gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_DISCOVERED,
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_RECOVERED:
		return true
	case gcsv1.FleetEventType_FLEET_EVENT_TYPE_VEHICLE_LOST,
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_HEARTBEAT_UPDATED,
		gcsv1.FleetEventType_FLEET_EVENT_TYPE_UNSPECIFIED:
		return false
	default:
		return false
	}
}

func (r *RateRequester) rates() []RateRequest {
	if r.Rates == nil {
		return DefaultRates
	}

	return r.Rates
}

func (r *RateRequester) log() *slog.Logger {
	if r.Log == nil {
		return slog.Default()
	}

	return r.Log
}

func (r *RateRequester) now() int64 {
	if r.Now == nil {
		return time.Now().UnixMilli()
	}

	return r.Now()
}
