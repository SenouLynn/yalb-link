package command

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	ErrQuarantined = errors.New("command: key quarantined after resolution")
	ErrNotPoisoned = errors.New("command: no unresolved command for this key")
	ErrStaleRef    = errors.New("command: transaction reference does not match the unresolved command")
	ErrObserved    = errors.New("command: observed arm state must be armed or disarmed")
	ErrNoRoute     = errors.New("command: no route to vehicle")
	ErrWriteFailed = errors.New("command: link write failed")
)

// QuarantineError reports how long a key stays closed to new commands after an
// operator resolution.
//
// It carries the remaining duration rather than a deadline so that no caller
// decides expiry from its own clock. The registry stays authoritative.
type QuarantineError struct{ RetryAfter time.Duration }

func (e *QuarantineError) Error() string {
	return fmt.Sprintf("%s: %s remaining", ErrQuarantined.Error(), e.RetryAfter)
}

func (e *QuarantineError) Is(target error) bool { return target == ErrQuarantined }

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

	// Quarantine overrides DefaultResolutionQuarantine. Tests inject it; it is
	// deliberately not derived from Timeout.
	Quarantine time.Duration

	// Epoch identifies this registry instance. Generated on first use unless a
	// test injects one.
	Epoch string

	mu sync.Mutex
	// inflight, poisoned, and quarantined are mutually exclusive per key.
	inflight map[key]*pending
	// poisoned retains the terminal transaction that caused the ambiguity, so
	// an operator who reloaded the page can be told exactly what to resolve.
	poisoned    map[key]*gcsv1.CommandTransaction
	quarantined map[key]time.Time
	nextID      uint32
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
	if poisoned, ok := r.poisoned[k]; ok {
		out := cloneTx(poisoned)
		r.mu.Unlock()
		// The retained transaction travels with the error so a browser that
		// has forgotten the ambiguity learns which one to resolve.
		return out, ErrPoisoned
	}
	if remaining, ok := r.remainingQuarantineLocked(k); ok {
		r.mu.Unlock()
		return nil, &QuarantineError{RetryAfter: remaining}
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
		IssuedAt:       timestamppb.New(now),
		RequestedState: armState(arm),
		OperatorLabel:  OperatorLabel,
		RegistryEpoch:  r.Epoch,
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
		p.tx.State = gcsv1.CommandState_COMMAND_STATE_SEND_FAILED
		p.tx.SettledAt = timestamppb.New(r.now())
		out := cloneTx(p.tx)
		r.poisoned[k] = cloneTx(p.tx)
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
	p.tx.State = state
	p.tx.SettledAt = timestamppb.New(r.now())
	if ack != nil {
		p.tx.Result = ack.GetResult()
		p.tx.ResultParam2 = ack.GetResultParam2()
	}
	out := cloneTx(p.tx)
	// Poison after the terminal state is stamped: the retained snapshot is what
	// the operator is later asked to resolve.
	if state == gcsv1.CommandState_COMMAND_STATE_TIMED_OUT || state == gcsv1.CommandState_COMMAND_STATE_CANCELLED {
		r.poisoned[k] = cloneTx(p.tx)
	}
	r.mu.Unlock()
	r.publish(context.WithoutCancel(ctx), out)
	return cloneTx(out), nil
}

// ResolveArm records an operator's attestation about an ambiguous arm/disarm
// transaction and moves its key from poisoned into a bounded quarantine.
//
// It never rewrites the transaction's terminal state. MAVLink cannot prove what
// the vehicle did, so the ambiguity stays a historical fact and the attestation
// is recorded beside it.
func (r *Registry) ResolveArm(
	ctx context.Context, sysID uint8, epoch string, txID uint32, observed gcsv1.ArmState,
) (*gcsv1.CommandTransaction, error) {
	if observed != gcsv1.ArmState_ARM_STATE_ARMED && observed != gcsv1.ArmState_ARM_STATE_DISARMED {
		return nil, ErrObserved
	}
	k := key{sysID: sysID, compID: codec.AutopilotComponentID, command: codec.CmdComponentArmDisarm}

	r.mu.Lock()
	r.initLocked()
	poisoned, ok := r.poisoned[k]
	if !ok {
		r.mu.Unlock()
		return nil, ErrNotPoisoned
	}
	if poisoned.GetRegistryEpoch() != epoch || poisoned.GetId() != txID {
		r.mu.Unlock()
		// A stale tab must not clear an ambiguity it never saw.
		return nil, ErrStaleRef
	}
	if _, busy := r.inflight[k]; busy {
		// Assertion, not an independent guarantee: Arm deletes inflight and
		// sets poisoned under one lock acquisition, so this cannot happen.
		r.mu.Unlock()
		return nil, ErrInFlight
	}

	now := r.now()
	until := now.Add(r.quarantine())
	out := cloneTx(poisoned)
	out.Resolution = &gcsv1.CommandResolution{
		ObservedState:   observed,
		OperatorLabel:   OperatorLabel,
		AttestedAt:      timestamppb.New(now),
		QuarantineUntil: timestamppb.New(until),
	}
	delete(r.poisoned, k)
	r.quarantined[k] = until
	r.mu.Unlock()

	// Publication is best effort, matching every other terminal snapshot here.
	// No production publisher can report failure, and rolling the quarantine
	// back on one would restore a poison that subscribers were already told was
	// resolved. A guaranteed audit record is a separate persistence concern.
	r.publish(context.WithoutCancel(ctx), out)
	return cloneTx(out), nil
}

// QuarantineRemaining reports the quarantine left on a vehicle's arm/disarm
// key, for callers that need it without attempting a command.
func (r *Registry) QuarantineRemaining(sysID uint8) (time.Duration, bool) {
	k := key{sysID: sysID, compID: codec.AutopilotComponentID, command: codec.CmdComponentArmDisarm}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initLocked()
	return r.remainingQuarantineLocked(k)
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
	_, quarantined := r.remainingQuarantineLocked(k)
	r.mu.Unlock()
	// The poisoned and quarantined checks are defensive rather than load
	// bearing: neither state permits a command in flight, so p is already nil
	// in both. They are kept so the sink still refuses if that ever changes.
	if p == nil || poisoned || quarantined || ack.GetResult() == gcsv1.MavResult_MAV_RESULT_IN_PROGRESS {
		return nil
	}
	select {
	case p.ack <- proto.Clone(ack).(*gcsv1.CommandAck):
	default:
	}
	return nil
}

// remainingQuarantineLocked reports the quarantine left on a key, removing it
// once it has expired. The caller holds mu.
func (r *Registry) remainingQuarantineLocked(k key) (time.Duration, bool) {
	until, ok := r.quarantined[k]
	if !ok {
		return 0, false
	}
	remaining := until.Sub(r.now())
	if remaining <= 0 {
		delete(r.quarantined, k)
		return 0, false
	}
	return remaining, true
}

func (r *Registry) initLocked() {
	if r.inflight == nil {
		r.inflight = make(map[key]*pending)
	}
	if r.poisoned == nil {
		r.poisoned = make(map[key]*gcsv1.CommandTransaction)
	}
	if r.quarantined == nil {
		r.quarantined = make(map[key]time.Time)
	}
	if r.Epoch == "" {
		r.Epoch = newEpoch()
	}
}

// newEpoch identifies one registry instance. Transaction ids are a per-process
// counter and poison clears on restart, so without this a client surviving a
// restart could resolve an unrelated transaction that reused the number.
func newEpoch() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail in practice; a time-derived fallback keeps
		// the registry usable rather than refusing every command.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func armState(arm bool) gcsv1.ArmState {
	if arm {
		return gcsv1.ArmState_ARM_STATE_ARMED
	}
	return gcsv1.ArmState_ARM_STATE_DISARMED
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
func (r *Registry) quarantine() time.Duration {
	if r.Quarantine > 0 {
		return r.Quarantine
	}
	return DefaultResolutionQuarantine
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
