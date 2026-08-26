package command

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
	gcsv1 "yalb.gcs/internal/gen/gcs/v1"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/vehicle"
)

type fakeSource struct {
	write func(message.Message) error
}

func (*fakeSource) Events() <-chan gomavlib.Event                       { return nil }
func (f *fakeSource) WriteTo(_ codec.LinkID, msg message.Message) error { return f.write(msg) }
func (*fakeSource) Close() error                                        { return nil }

type capture struct {
	mu  sync.Mutex
	txs []*gcsv1.CommandTransaction
}

func (c *capture) Publish(_ context.Context, ev vehicle.Event) error {
	if ev.Command != nil {
		c.mu.Lock()
		c.txs = append(c.txs, cloneTx(ev.Command))
		c.mu.Unlock()
	}
	return nil
}

func registryFor(t *testing.T, write func(message.Message) error) (*Registry, *capture) {
	t.Helper()
	table := routes.NewTable()
	table.Upsert(routes.Entry{Key: routes.Key{SysID: 1, CompID: codec.AutopilotComponentID}, Link: "test"}, time.Now().UnixMilli())
	c := &capture{}
	return &Registry{Source: &fakeSource{write: write}, Routes: table, Publisher: c,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Timeout: 30 * time.Millisecond}, c
}

func ackEvent(targetSystem, targetComponent uint32, result gcsv1.MavResult) vehicle.Event {
	return vehicle.Event{Protocol: &gcsv1.ProtocolEvent{
		VehicleId: &gcsv1.VehicleId{SystemId: 1, ComponentId: uint32(codec.AutopilotComponentID)},
		Payload: &gcsv1.ProtocolEvent_CommandAck{CommandAck: &gcsv1.CommandAck{
			Command: gcsv1.MavCmd_MAV_CMD_COMPONENT_ARM_DISARM, Result: result,
			TargetSystem: targetSystem, TargetComponent: targetComponent,
		}},
	}}
}

func TestArmPublishesImmutablePendingThenAccepted(t *testing.T) {
	var registry *Registry
	registry, published := registryFor(t, func(message.Message) error {
		return registry.Publish(context.Background(), ackEvent(255, 190, gcsv1.MavResult_MAV_RESULT_ACCEPTED))
	})
	tx, err := registry.Arm(context.Background(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if tx.GetState() != gcsv1.CommandState_COMMAND_STATE_ACCEPTED {
		t.Fatalf("state = %v", tx.GetState())
	}
	if len(published.txs) != 2 {
		t.Fatalf("published %d snapshots, want 2", len(published.txs))
	}
	if published.txs[0].GetState() != gcsv1.CommandState_COMMAND_STATE_PENDING {
		t.Fatal("first snapshot was mutated")
	}
	if published.txs[1].GetState() != gcsv1.CommandState_COMMAND_STATE_ACCEPTED {
		t.Fatal("terminal snapshot not accepted")
	}
}

func TestAckMustBeAddressedAndTimeoutPoisonsKey(t *testing.T) {
	var registry *Registry
	registry, _ = registryFor(t, func(message.Message) error {
		_ = registry.Publish(context.Background(), ackEvent(0, 0, gcsv1.MavResult_MAV_RESULT_ACCEPTED))
		_ = registry.Publish(context.Background(), ackEvent(42, 99, gcsv1.MavResult_MAV_RESULT_ACCEPTED))
		return nil
	})
	tx, err := registry.Arm(context.Background(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if tx.GetState() != gcsv1.CommandState_COMMAND_STATE_TIMED_OUT {
		t.Fatalf("state = %v", tx.GetState())
	}
	if _, err := registry.Arm(context.Background(), 1, false); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("second arm error = %v", err)
	}
	_ = registry.Publish(context.Background(), ackEvent(255, 190, gcsv1.MavResult_MAV_RESULT_ACCEPTED))
	if _, err := registry.Arm(context.Background(), 1, false); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("late ack unlocked key: %v", err)
	}
}

func TestPolicyNeverForceArms(t *testing.T) {
	msg, err := encodeArm(1, true)
	if err != nil {
		t.Fatal(err)
	}
	cmd := msg.(*ardupilotmega.MessageCommandLong)
	if cmd.Command != codec.CmdComponentArmDisarm || cmd.TargetComponent != codec.AutopilotComponentID {
		t.Fatalf("wrong target/command: %+v", cmd)
	}
	if cmd.Param2 != 0 || cmd.Param2 == codec.ForceArmMagic {
		t.Fatalf("param2 permits force arm: %v", cmd.Param2)
	}
}

func TestUncertainWriteFailurePoisonsAndPublishesOneState(t *testing.T) {
	registry, published := registryFor(t, func(message.Message) error { return errors.New("I/O uncertain") })
	tx, err := registry.Arm(context.Background(), 1, true)
	if !errors.Is(err, ErrWriteFailed) || tx.GetState() != gcsv1.CommandState_COMMAND_STATE_SEND_FAILED {
		t.Fatalf("tx=%v err=%v", tx, err)
	}
	if len(published.txs) != 1 {
		t.Fatalf("published %d snapshots", len(published.txs))
	}
	if _, err := registry.Arm(context.Background(), 1, true); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("not poisoned: %v", err)
	}
}
