import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  AttitudeSchema,
  GlobalPositionSchema,
  SystemStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import {
  fleetReducer,
  initialFleetState,
  TELEMETRY_TTL_MS,
  vehicleKey,
  type VehicleView,
} from '@/fleet/state';
import type { StreamEvent } from '@/stream/events';

import { hasDisplayValue, readFlight, stalenessFraction, UNAVAILABLE } from './readings';

const T0 = 1_700_000_000_000;

type TelemetryPayload = ReturnType<typeof create<typeof TelemetryEventSchema>>['payload'];

function telemetry(payload: TelemetryPayload, atMs: number): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, {
      vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
      payload,
    }),
  };
}

function attitudeAt(rollRad: number, pitchRad: number, atMs: number): StreamEvent {
  return telemetry(
    { case: 'attitude', value: create(AttitudeSchema, { rollRad, pitchRad, yawRad: 0 }) },
    atMs,
  );
}

function vfrAt(groundspeedMS: number, atMs: number): StreamEvent {
  return telemetry(
    {
      case: 'vfrHud',
      value: create(VfrHudSchema, { groundspeedMS, headingDeg: 90, climbMS: 1 }),
    },
    atMs,
  );
}

function sysStatusAt(voltageBatteryMv: number, atMs: number): StreamEvent {
  return telemetry(
    {
      case: 'systemStatus',
      value: create(SystemStatusSchema, { voltageBatteryMv, batteryRemainingPct: 55 }),
    },
    atMs,
  );
}

function viewOf(...events: StreamEvent[]): VehicleView {
  const state = events.reduce(
    (acc, event) => fleetReducer(acc, { type: 'stream', event }),
    initialFleetState,
  );

  const view = state.vehicles[vehicleKey(1, 1)];
  if (view === undefined) throw new Error('no vehicle in state');

  return view;
}

describe('readFlight', () => {
  it('reports every instrument as unavailable before anything arrives', () => {
    const view = viewOf(attitudeAt(0.1, 0, T0));
    const readings = readFlight(view, T0);

    expect(readings.position.state).toBe('unavailable');
    expect(readings.position.value).toBeNull();
    expect(readings.battery.state).toBe('unavailable');
  });

  it('marks a just-received value live and carries its provenance', () => {
    const readings = readFlight(viewOf(attitudeAt(0.25, -0.1, T0)), T0);

    expect(readings.attitude.state).toBe('live');
    expect(readings.attitude.source).toBe('ATTITUDE');
    expect(readings.attitude.ageMs).toBe(0);
    expect(readings.attitude.value?.rollDeg).toBeCloseTo(14.32, 1);
  });

  it('keeps stale provenance but withholds the value from display', () => {
    const readings = readFlight(viewOf(attitudeAt(0.25, 0, T0)), T0 + TELEMETRY_TTL_MS + 1);

    expect(readings.attitude.state).toBe('stale');
    expect(readings.attitude.value).not.toBeNull();
    expect(hasDisplayValue(readings.attitude)).toBe(false);
    expect(readings.attitude.ageMs).toBe(TELEMETRY_TTL_MS + 1);
  });

  it('ages each instrument independently by the family it resolved from', () => {
    const view = viewOf(attitudeAt(0.25, 0, T0), vfrAt(6, T0 + 4_800));

    const readings = readFlight(view, T0 + 5_200);

    expect(readings.attitude.state).toBe('stale');
    expect(readings.heading.state).toBe('live');
    expect(readings.heading.source).toBe('VFR_HUD');
  });

  it('never reports a zero reading for a sensor that has not been heard from', () => {
    // A level aircraft and a missing attitude both look like roll 0; only one
    // of them may be rendered as a number.
    const missing = readFlight(viewOf(vfrAt(6, T0)), T0);
    expect(missing.attitude.state).toBe('unavailable');
    expect(hasDisplayValue(missing.attitude)).toBe(false);

    const level = readFlight(viewOf(attitudeAt(0, 0, T0)), T0);
    expect(level.attitude.state).toBe('live');
    expect(hasDisplayValue(level.attitude)).toBe(true);
    expect(level.attitude.value?.rollDeg).toBe(0);
  });

  it('resolves the battery through the sample the reducer accumulated', () => {
    const readings = readFlight(viewOf(sysStatusAt(12_400, T0)), T0);

    expect(readings.battery.state).toBe('live');
    expect(readings.battery.value?.voltageV).toBeCloseTo(12.4);
    expect(readings.battery.value?.remainingPct).toBe(55);
  });

  it('identifies the altitude datum rather than merging the two', () => {
    const view = viewOf(
      telemetry(
        {
          case: 'globalPosition',
          value: create(GlobalPositionSchema, {
            latDeg: 37.7,
            lonDeg: -122.4,
            altRelativeM: 25,
            altMslM: 132,
          }),
        },
        T0,
      ),
    );

    const readings = readFlight(view, T0);

    expect(readings.position.value?.altRef).toBe('RELATIVE');
    expect(readings.position.value?.altM).toBeCloseTo(25);
  });
});

describe('stalenessFraction', () => {
  it('is 0 for a value that just arrived and 1 at the TTL', () => {
    const fresh = readFlight(viewOf(attitudeAt(0, 0, T0)), T0);
    expect(stalenessFraction(fresh.attitude)).toBe(0);

    const expired = readFlight(viewOf(attitudeAt(0, 0, T0)), T0 + TELEMETRY_TTL_MS);
    expect(stalenessFraction(expired.attitude)).toBe(1);
  });

  it('is halfway through the TTL at half the TTL', () => {
    const half = readFlight(viewOf(attitudeAt(0, 0, T0)), T0 + TELEMETRY_TTL_MS / 2);
    expect(stalenessFraction(half.attitude)).toBeCloseTo(0.5);
  });

  it('clamps rather than exceeding 1 for a long-dead family', () => {
    const old = readFlight(viewOf(attitudeAt(0, 0, T0)), T0 + TELEMETRY_TTL_MS * 10);
    expect(stalenessFraction(old.attitude)).toBe(1);
  });

  it('treats an unavailable reading as fully stale', () => {
    expect(stalenessFraction(UNAVAILABLE)).toBe(1);
  });
});
