import { create, toJsonString } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { AttitudeSchema, TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';
import { MavType } from '@/gen/gcs/v1/types_pb';
import { HeartbeatStateSchema, VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

import { parseStreamEvent } from './parse';

const NOW_MS = 1_700_000_000_000;

function id(systemId: number, componentId: number) {
  return create(VehicleIdSchema, { systemId, componentId });
}

describe('parseStreamEvent', () => {
  it('reconstructs a fleet event from protobuf JSON', () => {
    const data = toJsonString(
      FleetEventSchema,
      create(FleetEventSchema, {
        type: FleetEventType.VEHICLE_DISCOVERED,
        vehicleId: id(3, 1),
        heartbeat: create(HeartbeatStateSchema, { type: MavType.QUADROTOR, armed: true }),
      }),
    );

    const parsed = parseStreamEvent('fleet', data, NOW_MS);

    expect(parsed?.kind).toBe('fleet');
    if (parsed?.kind !== 'fleet') throw new Error('unreachable');

    expect(parsed.event.type).toBe(FleetEventType.VEHICLE_DISCOVERED);
    expect(parsed.event.vehicleId?.systemId).toBe(3);
    expect(parsed.event.heartbeat?.armed).toBe(true);
    expect(parsed.receivedAtMs).toBe(NOW_MS);
  });

  it('reconstructs a telemetry event and keeps its oneof case', () => {
    const data = toJsonString(
      TelemetryEventSchema,
      create(TelemetryEventSchema, {
        vehicleId: id(1, 1),
        payload: { case: 'attitude', value: create(AttitudeSchema, { rollRad: -0.4 }) },
      }),
    );

    const parsed = parseStreamEvent('telemetry', data, NOW_MS);

    if (parsed?.kind !== 'telemetry') throw new Error('expected a telemetry event');

    expect(parsed.event.payload.case).toBe('attitude');
    if (parsed.event.payload.case !== 'attitude') throw new Error('unreachable');
    expect(parsed.event.payload.value.rollRad).toBeCloseTo(-0.4);
  });

  it('preserves an explicit zero rather than dropping the field', () => {
    // The backend emits default values precisely so this stays distinguishable
    // from a field that was never sent.
    const parsed = parseStreamEvent(
      'telemetry',
      '{"vehicleId":{"systemId":1,"componentId":1},"attitude":{"rollRad":0,"pitchRad":0,"yawRad":0}}',
      NOW_MS,
    );

    if (parsed?.kind !== 'telemetry') throw new Error('expected a telemetry event');
    if (parsed.event.payload.case !== 'attitude') throw new Error('expected attitude');

    expect(parsed.event.payload.value.rollRad).toBe(0);
  });

  it('returns null for an unknown event name instead of throwing', () => {
    expect(parseStreamEvent('warning', '{}', NOW_MS)).toBeNull();
  });

  it('returns null for malformed JSON so one bad frame cannot kill the stream', () => {
    expect(parseStreamEvent('telemetry', '{not json', NOW_MS)).toBeNull();
    expect(parseStreamEvent('fleet', '', NOW_MS)).toBeNull();
  });
});
