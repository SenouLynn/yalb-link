// Package mission downloads a vehicle's onboard mission into a validated
// snapshot.
//
// The MAVLink mission protocol is a request/response transaction over a link
// that is shared by every vehicle, every component, every mission type, and
// every ground station talking to them. Nothing about a response says which
// transaction it belongs to except the fields on the response itself, so the
// work here is mostly correlation: deciding what to ignore.
package mission

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

// Terminal download failures. Each is distinct because a caller has to be able
// to tell "ask again" from "this vehicle said no" from "we never reached it".
var (
	ErrInFlight    = errors.New("mission: download already in flight for this vehicle and mission type")
	ErrNoRoute     = errors.New("mission: no route to vehicle")
	ErrWriteFailed = errors.New("mission: link write failed")
	ErrTimeout     = errors.New("mission: vehicle stopped responding")
	ErrCancelled   = errors.New("mission: download cancelled")
	ErrRejected    = errors.New("mission: vehicle rejected the transfer")
)

// DefaultTimeout bounds the wait for any single response.
//
// It is per-response, not per-download: a large mission legitimately takes many
// round trips, and a deadline over the whole transfer would fail slow-but-
// healthy links while still tolerating a vehicle that had gone quiet.
const DefaultTimeout = 5 * time.Second

// eventBuffer sizes each download's inbound queue.
//
// A well-behaved vehicle sends one response per request, so anything beyond the
// first slot is slack for duplicates and retransmissions arriving while the
// coordinator is between requests. Publish never blocks on a full queue.
const eventBuffer = 8

// RejectedError carries the vehicle's own reason for refusing a transfer.
type RejectedError struct{ Result gcsv1.MavMissionResult }

func (e *RejectedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrRejected.Error(), e.Result)
}

func (e *RejectedError) Is(target error) bool { return target == ErrRejected }

// key identifies one transaction slot.
//
// Mission type is part of the key because a vehicle can hold a flight plan, a
// geofence, and a rally set at once, and answers for all three over one link.
// Component ID is part of it because components sharing a system ID hold
// separate missions.
type key struct {
	sysID, compID uint8
	missionType   uint32
}

type download struct {
	events chan *gcsv1.ProtocolEvent
}

// Coordinator owns the in-flight download slots and correlates responses.
//
// The zero value is not usable: Source and Routes must be set.
type Coordinator struct {
	Source  codec.FrameSource
	Routes  *routes.Table
	Log     *slog.Logger
	Now     func() time.Time
	Timeout time.Duration

	mu       sync.Mutex
	inflight map[key]*download
}

var _ interface {
	Publish(context.Context, vehicle.Event) error
} = (*Coordinator)(nil)

// Download reads one vehicle's mission of the given type into a snapshot.
//
// It is a single attempt. A response that never arrives is a timeout rather
// than a retry, because the coordinator cannot tell a dropped request from a
// vehicle that is busy, and re-requesting a sequence the vehicle is already
// answering is how a transfer desynchronises.
func (c *Coordinator) Download(
	ctx context.Context,
	target codec.Target,
	missionType uint32,
) (*gcsv1.MissionSnapshot, error) {
	if c.Source == nil || c.Routes == nil {
		return nil, ErrNoRoute
	}

	k := key{sysID: target.SystemID, compID: target.ComponentID, missionType: missionType}

	link, err := c.Routes.Resolve(
		routes.Key{SysID: k.sysID, CompID: k.compID},
		c.now().UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoRoute, err)
	}

	d, err := c.claim(k)
	if err != nil {
		return nil, err
	}

	defer c.release(k)

	return c.run(ctx, d, link, target, missionType)
}

// run drives the transaction once the slot is held.
func (c *Coordinator) run(
	ctx context.Context,
	d *download,
	link codec.LinkID,
	target codec.Target,
	missionType uint32,
) (*gcsv1.MissionSnapshot, error) {
	if err := c.write(link, codec.EncodeMissionRequestList(target, missionType)); err != nil {
		return nil, err
	}

	count, err := c.awaitCount(ctx, d)
	if err != nil {
		return nil, err
	}

	items := make([]*gcsv1.MissionItem, 0, count)

	// Request each advertised sequence exactly once, in order. The vehicle
	// decides how many there are; we never infer the end from anything else.
	for seq := uint32(0); seq < count; seq++ {
		if err := c.write(link, codec.EncodeMissionRequestInt(target, uint16(seq), missionType)); err != nil {
			return nil, err
		}

		item, err := c.awaitItem(ctx, d, seq, count)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	// The vehicle holds the transaction open until it is acknowledged, empty
	// missions included. Failing to send this leaves the next download to
	// collide with a transfer the vehicle still thinks is running.
	if err := c.write(link, codec.EncodeMissionAck(
		target,
		uint32(gcsv1.MavMissionResult_MAV_MISSION_ACCEPTED),
		missionType,
	)); err != nil {
		return nil, err
	}

	return &gcsv1.MissionSnapshot{
		VehicleId: &gcsv1.VehicleId{
			SystemId:    uint32(target.SystemID),
			ComponentId: uint32(target.ComponentID),
		},
		MissionType: gcsv1.MavMissionType(missionType),
		Items:       items,
		ObservedAt:  timestamppb.New(c.now()),
	}, nil
}

// awaitCount waits for the count that opens the transfer.
func (c *Coordinator) awaitCount(ctx context.Context, d *download) (uint32, error) {
	for {
		ev, err := c.next(ctx, d)
		if err != nil {
			return 0, err
		}

		if ack := ev.GetMissionAck(); ack != nil {
			// A vehicle that refuses the request answers with an ACK instead of
			// a count. ACCEPTED here is not an answer to anything we asked, so
			// it is ignored rather than treated as an empty mission.
			if ack.GetResult() != gcsv1.MavMissionResult_MAV_MISSION_ACCEPTED {
				return 0, &RejectedError{Result: ack.GetResult()}
			}

			continue
		}

		if count := ev.GetMissionCount(); count != nil {
			return count.GetCount(), nil
		}
	}
}

// awaitItem waits for the one item that answers the outstanding request.
//
// Anything else is dropped: a duplicate of an item already stored, an item for
// a sequence we are not currently asking for, or a sequence outside the
// advertised range. Accepting any of those would either reorder the route or
// let a stale retransmission overwrite a good item.
func (c *Coordinator) awaitItem(
	ctx context.Context,
	d *download,
	want, count uint32,
) (*gcsv1.MissionItem, error) {
	for {
		ev, err := c.next(ctx, d)
		if err != nil {
			return nil, err
		}

		if ack := ev.GetMissionAck(); ack != nil {
			if ack.GetResult() != gcsv1.MavMissionResult_MAV_MISSION_ACCEPTED {
				return nil, &RejectedError{Result: ack.GetResult()}
			}

			continue
		}

		item := ev.GetMissionItem()
		if item == nil {
			continue
		}

		if item.GetSeq() >= count || item.GetSeq() != want {
			c.log().Debug("mission: ignoring unexpected item sequence",
				"seq", item.GetSeq(), "want", want, "count", count)

			continue
		}

		return item, nil
	}
}

// next returns the next correlated response, or the reason there will not be
// one. Timeout is measured per response.
func (c *Coordinator) next(ctx context.Context, d *download) (*gcsv1.ProtocolEvent, error) {
	timer := time.NewTimer(c.timeout())
	defer timer.Stop()

	select {
	case ev := <-d.events:
		return ev, nil
	case <-timer.C:
		return nil, ErrTimeout
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrCancelled, ctx.Err())
	}
}

func (c *Coordinator) write(link codec.LinkID, msg message.Message) error {
	if err := c.Source.WriteTo(link, msg); err != nil {
		if errors.Is(err, codec.ErrUnknownLink) {
			return fmt.Errorf("%w: %v", ErrNoRoute, err)
		}

		return fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	return nil
}

// Publish receives fold output and hands mission responses to the download they
// belong to, if any.
//
// Every rejection here is a correlation rule, not a filter for tidiness. The
// envelope's vehicle_id says who sent the response; the payload's target says
// which ground station it was addressed to. A response matching on one but not
// the other belongs to somebody else's transfer.
func (c *Coordinator) Publish(_ context.Context, ev vehicle.Event) error {
	if ev.Protocol == nil || ev.Protocol.GetVehicleId() == nil {
		return nil
	}

	missionType, target, ok := missionResponse(ev.Protocol)
	if !ok {
		return nil
	}

	// Addressed to this GCS, or it is an answer to another ground station.
	if target.SystemID != codec.GCSSystemID || target.ComponentID != codec.GCSComponentID {
		return nil
	}

	id := ev.Protocol.GetVehicleId()
	k := key{
		sysID:       uint8(id.GetSystemId()),
		compID:      uint8(id.GetComponentId()),
		missionType: missionType,
	}

	c.mu.Lock()
	d := c.inflight[k]
	c.mu.Unlock()

	if d == nil {
		return nil
	}

	// Never block the fold. A download that has fallen this far behind is
	// already failing on its own timeout.
	select {
	case d.events <- proto.Clone(ev.Protocol).(*gcsv1.ProtocolEvent):
	default:
		c.log().Warn("mission: dropping response, download queue full",
			"system_id", k.sysID, "component_id", k.compID, "mission_type", k.missionType)
	}

	return nil
}

// missionResponse reports the mission type and addressed GCS of a mission
// transaction payload, and whether the event was one at all.
func missionResponse(ev *gcsv1.ProtocolEvent) (uint32, codec.Target, bool) {
	switch {
	case ev.GetMissionCount() != nil:
		m := ev.GetMissionCount()

		return uint32(m.GetMissionType()), codec.Target{
			SystemID:    uint8(m.GetTargetSystem()),
			ComponentID: uint8(m.GetTargetComponent()),
		}, true

	case ev.GetMissionItem() != nil:
		m := ev.GetMissionItem()

		return uint32(m.GetMissionType()), codec.Target{
			SystemID:    uint8(m.GetTargetSystem()),
			ComponentID: uint8(m.GetTargetComponent()),
		}, true

	case ev.GetMissionAck() != nil:
		m := ev.GetMissionAck()

		return uint32(m.GetMissionType()), codec.Target{
			SystemID:    uint8(m.GetTargetSystem()),
			ComponentID: uint8(m.GetTargetComponent()),
		}, true
	}

	return 0, codec.Target{}, false
}

func (c *Coordinator) claim(k key) (*download, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.inflight == nil {
		c.inflight = make(map[key]*download)
	}

	if _, ok := c.inflight[k]; ok {
		return nil, ErrInFlight
	}

	d := &download{events: make(chan *gcsv1.ProtocolEvent, eventBuffer)}
	c.inflight[k] = d

	return d, nil
}

// release frees the slot. Every terminal path runs it, so a failed download
// never leaves a vehicle permanently un-downloadable.
func (c *Coordinator) release(k key) {
	c.mu.Lock()
	delete(c.inflight, k)
	c.mu.Unlock()
}

func (c *Coordinator) inFlightCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.inflight)
}

func (c *Coordinator) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}

	return time.Now()
}

func (c *Coordinator) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}

	return DefaultTimeout
}

func (c *Coordinator) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}

	return slog.Default()
}
