package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

var (
	ErrInFlight    = errors.New("command: command already in flight")
	ErrPoisoned    = errors.New("command: previous command unresolved")
	ErrNoRoute     = errors.New("command: no route to vehicle")
	ErrWriteFailed = errors.New("command: link write failed")
)

type Publisher interface {
	Publish(context.Context, vehicle.Event) error
}

type key struct {
	sysID, compID uint8
	command       uint32
}

type pending struct {
	tx  *gcsv1.CommandTransaction
	ack chan *gcsv1.CommandAck
}

type Registry struct {
	Source    codec.FrameSource
	Routes    *routes.Table
	Publisher Publisher
	Log       *slog.Logger
	Now       func() time.Time
	Timeout   time.Duration

	mu       sync.Mutex
	inflight map[key]*pending
	poisoned map[key]struct{}
	nextID   uint32
}

func (r *Registry) Arm(ctx context.Context, sysID uint8, arm bool) (*gcsv1.CommandTransaction, error) {
	msg, err := encodeArm(sysID, arm)
	if err != nil {
		return nil, err
	}
	if r.Source == nil || r.Routes == nil {
		return nil, ErrNoRoute
	}
	k := key{sysID: sysID, compID: codec.AutopilotComponentID, command: codec.CmdComponentArmDisarm}
	link, err := r.Routes.Resolve(routes.Key{SysID: k.sysID, CompID: k.compID}, r.now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoRoute, err)
	}

	r.mu.Lock()
	r.initLocked()
	if _, ok := r.poisoned[k]; ok {
		r.mu.Unlock()
		return nil, ErrPoisoned
	}
	if _, ok := r.inflight[k]; ok {
		r.mu.Unlock()
		return nil, ErrInFlight
	}
	r.nextID++
	now := r.now()
	p := &pending{tx: &gcsv1.CommandTransaction{
		Id:        r.nextID,
		VehicleId: &gcsv1.VehicleId{SystemId: uint32(k.sysID), ComponentId: uint32(k.compID)},
		Command:   gcsv1.MavCmd(k.command), State: gcsv1.CommandState_COMMAND_STATE_PENDING,
		IssuedAt: timestamppb.New(now),
	}, ack: make(chan *gcsv1.CommandAck, 1)}
	r.inflight[k] = p
	r.mu.Unlock()

	if err := r.Source.WriteTo(link, msg); err != nil {
		r.mu.Lock()
		delete(r.inflight, k)
		if errors.Is(err, codec.ErrUnknownLink) {
			r.mu.Unlock()
			return nil, fmt.Errorf("%w: %v", ErrNoRoute, err)
		}
		r.poisoned[k] = struct{}{}
		p.tx.State = gcsv1.CommandState_COMMAND_STATE_SEND_FAILED
		p.tx.SettledAt = timestamppb.New(r.now())
		out := cloneTx(p.tx)
		r.mu.Unlock()
		r.publish(context.WithoutCancel(ctx), out)
		return cloneTx(out), fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	r.publish(context.WithoutCancel(ctx), cloneTx(p.tx))
	timer := time.NewTimer(r.timeout())
	defer timer.Stop()

	var state gcsv1.CommandState
	var ack *gcsv1.CommandAck
	select {
	case ack = <-p.ack:
		if ack.GetResult() == gcsv1.MavResult_MAV_RESULT_ACCEPTED {
			state = gcsv1.CommandState_COMMAND_STATE_ACCEPTED
		} else {
			state = gcsv1.CommandState_COMMAND_STATE_REJECTED
		}
	case <-timer.C:
		state = gcsv1.CommandState_COMMAND_STATE_TIMED_OUT
	case <-ctx.Done():
		state = gcsv1.CommandState_COMMAND_STATE_CANCELLED
	}

	r.mu.Lock()
	delete(r.inflight, k)
	if state == gcsv1.CommandState_COMMAND_STATE_TIMED_OUT || state == gcsv1.CommandState_COMMAND_STATE_CANCELLED {
		r.poisoned[k] = struct{}{}
	}
	p.tx.State = state
	p.tx.SettledAt = timestamppb.New(r.now())
	if ack != nil {
		p.tx.Result = ack.GetResult()
		p.tx.ResultParam2 = ack.GetResultParam2()
	}
	out := cloneTx(p.tx)
	r.mu.Unlock()
	r.publish(context.WithoutCancel(ctx), out)
	return cloneTx(out), nil
}

func (r *Registry) Publish(ctx context.Context, ev vehicle.Event) error {
	ack := ev.Protocol.GetCommandAck()
	if ack == nil || ev.Protocol.GetVehicleId() == nil {
		return nil
	}
	if ack.GetTargetSystem() != uint32(codec.GCSSystemID) || ack.GetTargetComponent() != uint32(codec.GCSComponentID) {
		return nil
	}
	id := ev.Protocol.GetVehicleId()
	k := key{sysID: uint8(id.GetSystemId()), compID: uint8(id.GetComponentId()), command: uint32(ack.GetCommand())}
	r.mu.Lock()
	r.initLocked()
	p := r.inflight[k]
	_, poisoned := r.poisoned[k]
	r.mu.Unlock()
	if p == nil || poisoned || ack.GetResult() == gcsv1.MavResult_MAV_RESULT_IN_PROGRESS {
		return nil
	}
	select {
	case p.ack <- proto.Clone(ack).(*gcsv1.CommandAck):
	default:
	}
	return nil
}

func (r *Registry) initLocked() {
	if r.inflight == nil {
		r.inflight = make(map[key]*pending)
	}
	if r.poisoned == nil {
		r.poisoned = make(map[key]struct{})
	}
}
func (r *Registry) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
func (r *Registry) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultTimeout
}
func cloneTx(tx *gcsv1.CommandTransaction) *gcsv1.CommandTransaction {
	return proto.Clone(tx).(*gcsv1.CommandTransaction)
}
func (r *Registry) publish(ctx context.Context, tx *gcsv1.CommandTransaction) {
	if r.Publisher != nil {
		if err := r.Publisher.Publish(ctx, vehicle.Event{Command: cloneTx(tx)}); err != nil {
			r.log().ErrorContext(ctx, "command publication failed", "id", tx.GetId(), "err", err)
		}
	}
}
func (r *Registry) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}
