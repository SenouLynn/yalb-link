import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import {
  AttitudeSchema,
  BatteryStatusSchema,
  GlobalPositionSchema,
  GpsRawSchema,
  SystemStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { MavType } from '@/gen/gcs/v1/types_pb';
import { HeartbeatStateSchema, VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import { resolveBattery } from '@/logic/battery';
import { resolvePosition } from '@/logic/position';
import type { StreamEvent } from '@/stream/events';

import {
  fleetReducer,
  initialFleetState,
  isActive,
  isFamilyFresh,
  TELEMETRY_TTL_MS,
  vehicleKey,
  type FleetState,
} from './state';

const T0 = 1_700_000_000_000;
type TelemetryPayload = ReturnType<typeof create<typeof TelemetryEventSchema>>['payload'];

function id(systemId: number, componentId: number) {
  return create(VehicleIdSchema, { systemId, componentId });
}

function fleetEvent(
  type: FleetEventType,
  sysId: number,
  compId: number,
  atMs = T0,
  heartbeat = create(HeartbeatStateSchema, {
    type: MavType.QUADROTOR,
    armed: true,
    customMode: 4,
  }),
): StreamEvent {
  return {
    kind: 'fleet',
    receivedAtMs: atMs,
    event: create(FleetEventSchema, { type, vehicleId: id(sysId, compId), heartbeat }),
  };
}

function attitude(sysId: number, compId: number, rollRad: number, atMs = T0): StreamEvent {
  return telemetryEvent(
    sysId,
    compId,
    { case: 'attitude', value: create(AttitudeSchema, { rollRad }) },
    atMs,
  );
}

function telemetryEvent(
  sysId: number,
  compId: number,
  payload: TelemetryPayload,
  atMs = T0,
): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, {
      vehicleId: id(sysId, compId),
      payload,
    }),
  };
}

function vfrHud(sysId: number, compId: number, groundspeedMS: number, atMs = T0): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, {
      vehicleId: id(sysId, compId),
      payload: { case: 'vfrHud', value: create(VfrHudSchema, { groundspeedMS }) },
    }),
  };
}

function globalPosition(sysId: number, compId: number, altRelativeM: number, atMs = T0): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, {
      vehicleId: id(sysId, compId),
      payload: {
        case: 'globalPosition',
        value: create(GlobalPositionSchema, { latDeg: 37.7, lonDeg: -122.4, altRelativeM }),
      },
    }),
  };
}

/** Applies a sequence of stream events to the empty state. */
function reduce(...events: StreamEvent[]): FleetState {
  return events.reduce(
    (state, event) => fleetReducer(state, { type: 'stream', event }),
    initialFleetState,
  );
}

describe('fleetReducer', () => {
  it('starts with an empty fleet and no selection', () => {
    expect(initialFleetState.order).toEqual([]);
    expect(initialFleetState.selected).toBeNull();
  });

  it('creates a vehicle from a fleet event', () => {
    const state = reduce(fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 1, 1));

    const view = state.vehicles[vehicleKey(1, 1)];

    expect(view?.sysId).toBe(1);
    expect(view?.compId).toBe(1);
    expect(view?.lifecycle).toBe(FleetEventType.VEHICLE_DISCOVERED);
    expect(view?.heartbeat?.armed).toBe(true);
  });

  it('creates a vehicle from telemetry that arrives before any fleet event', () => {
    const state = reduce(attitude(2, 1, 0.3));

    expect(state.vehicles[vehicleKey(2, 1)]?.sample.rollRad).toBeCloseTo(0.3);
    expect(state.selected).toBe(vehicleKey(2, 1));
  });

  it('keys by the full identity, so a gimbal is not the autopilot', () => {
    const state = reduce(attitude(1, 1, 0.1), attitude(1, 154, 0.9));

    expect(state.order).toEqual([vehicleKey(1, 1), vehicleKey(1, 154)]);
    expect(state.vehicles[vehicleKey(1, 1)]?.sample.rollRad).toBeCloseTo(0.1);
    expect(state.vehicles[vehicleKey(1, 154)]?.sample.rollRad).toBeCloseTo(0.9);
  });

  it('accumulates partial events across families into one sample', () => {
    const state = reduce(attitude(1, 1, 0.2), vfrHud(1, 1, 7.5), globalPosition(1, 1, 30));

    const sample = state.vehicles[vehicleKey(1, 1)]?.sample;

    expect(sample?.rollRad).toBeCloseTo(0.2);
    expect(sample?.groundspeedMps).toBeCloseTo(7.5);
    expect(sample?.globalAltRelativeM).toBeCloseTo(30);
  });

  it('keeps vehicles isolated from each other', () => {
    const state = reduce(attitude(1, 1, 0.2), attitude(2, 1, -0.6), vfrHud(2, 1, 11));

    expect(state.vehicles[vehicleKey(1, 1)]?.sample.rollRad).toBeCloseTo(0.2);
    expect(state.vehicles[vehicleKey(1, 1)]?.sample.groundspeedMps).toBeUndefined();
    expect(state.vehicles[vehicleKey(2, 1)]?.sample.groundspeedMps).toBeCloseTo(11);
  });

  it('orders vehicles by identity regardless of arrival order', () => {
    const state = reduce(
      attitude(3, 1, 0),
      attitude(1, 190, 0),
      attitude(2, 1, 0),
      attitude(1, 1, 0),
    );

    expect(state.order).toEqual([
      vehicleKey(1, 1),
      vehicleKey(1, 190),
      vehicleKey(2, 1),
      vehicleKey(3, 1),
    ]);
  });

  it('folds heartbeat identity into the sample the display reads', () => {
    const state = reduce(fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 1, 1));

    const sample = state.vehicles[vehicleKey(1, 1)]?.sample;

    expect(sample?.armed).toBe(true);
    expect(sample?.customMode).toBe(4);
    expect(sample?.vehicleType).toBe(MavType.QUADROTOR);
  });

  it('keeps the last known heartbeat when a later event omits one', () => {
    const state = reduce(
      fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 1, 1),
      {
        kind: 'fleet',
        receivedAtMs: T0 + 1000,
        event: create(FleetEventSchema, {
          type: FleetEventType.VEHICLE_LOST,
          vehicleId: id(1, 1),
        }),
      },
    );

    const view = state.vehicles[vehicleKey(1, 1)];

    expect(view?.lifecycle).toBe(FleetEventType.VEHICLE_LOST);
    expect(view?.heartbeat?.armed).toBe(true);
  });

  it('tracks connection state separately from vehicle state', () => {
    let state = reduce(fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 1, 1));
    expect(state.connected).toBe(false);

    state = fleetReducer(state, {
      type: 'stream',
      event: { kind: 'connection', connected: true, receivedAtMs: T0 },
    });

    expect(state.connected).toBe(true);
    expect(state.vehicles[vehicleKey(1, 1)]?.lifecycle).toBe(FleetEventType.VEHICLE_DISCOVERED);
  });

  it('ignores events with no vehicle identity', () => {
    const state = fleetReducer(initialFleetState, {
      type: 'stream',
      event: {
        kind: 'telemetry',
        receivedAtMs: T0,
        event: create(TelemetryEventSchema, {
          payload: { case: 'attitude', value: create(AttitudeSchema, {}) },
        }),
      },
    });

    expect(state).toBe(initialFleetState);
  });
});

describe('selection', () => {
  it('auto-selects the first active vehicle', () => {
    const state = reduce(attitude(3, 1, 0), attitude(1, 1, 0));

    expect(state.selected).toBe(vehicleKey(1, 1));
  });

  it('skips a lost vehicle when choosing automatically', () => {
    const state = reduce(
      fleetEvent(FleetEventType.VEHICLE_LOST, 1, 1),
      fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 2, 1),
    );

    expect(state.selected).toBe(vehicleKey(2, 1));
  });

  it('preserves the operator selection across later updates', () => {
    let state = reduce(attitude(1, 1, 0), attitude(2, 1, 0));

    state = fleetReducer(state, { type: 'select', key: vehicleKey(2, 1) });
    expect(state.selected).toBe(vehicleKey(2, 1));

    state = fleetReducer(state, { type: 'stream', event: attitude(1, 1, 0.5) });
    expect(state.selected).toBe(vehicleKey(2, 1));
  });

  it('holds the selection on a vehicle that goes lost', () => {
    let state = reduce(attitude(1, 1, 0), attitude(2, 1, 0));

    state = fleetReducer(state, { type: 'select', key: vehicleKey(2, 1) });
    state = fleetReducer(state, {
      type: 'stream',
      event: fleetEvent(FleetEventType.VEHICLE_LOST, 2, 1, T0 + 1000),
    });

    expect(state.selected).toBe(vehicleKey(2, 1));
  });

  it('ignores a selection of an unknown vehicle', () => {
    const state = reduce(attitude(1, 1, 0));

    expect(fleetReducer(state, { type: 'select', key: vehicleKey(9, 9) })).toBe(state);
  });

  it('falls back to the first vehicle when every one is lost', () => {
    const state = reduce(
      fleetEvent(FleetEventType.VEHICLE_LOST, 2, 1),
      fleetEvent(FleetEventType.VEHICLE_LOST, 1, 1),
    );

    expect(state.selected).toBe(vehicleKey(1, 1));
  });
});

describe('per-family freshness', () => {
  it('records a timestamp per family, not per vehicle', () => {
    const state = reduce(attitude(1, 1, 0.2, T0), vfrHud(1, 1, 7, T0 + 4000));

    const view = state.vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(view.familySeenMs['ATTITUDE']).toBe(T0);
    expect(view.familySeenMs['VFR_HUD']).toBe(T0 + 4000);
  });

  it('ages one family out while another stays fresh', () => {
    const state = reduce(attitude(1, 1, 0.2, T0), vfrHud(1, 1, 7, T0 + 4000));

    const view = state.vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    const now = T0 + 6000;

    expect(isFamilyFresh(view, 'ATTITUDE', now)).toBe(false);
    expect(isFamilyFresh(view, 'VFR_HUD', now)).toBe(true);
  });

  it('uses the strict boundary: exactly at the TTL is stale', () => {
    const state = reduce(attitude(1, 1, 0.2, T0));

    const view = state.vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(isFamilyFresh(view, 'ATTITUDE', T0 + TELEMETRY_TTL_MS - 1)).toBe(true);
    expect(isFamilyFresh(view, 'ATTITUDE', T0 + TELEMETRY_TTL_MS)).toBe(false);
  });

  it('treats a family that has never arrived as stale, not fresh', () => {
    const state = reduce(attitude(1, 1, 0.2, T0));

    const view = state.vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(isFamilyFresh(view, 'GPS_RAW_INT', T0)).toBe(false);
  });

  it('rejects a non-finite family timestamp through the shared freshness helper', () => {
    const state = reduce(attitude(1, 1, 0.2, T0));
    const current = state.vehicles[vehicleKey(1, 1)];
    if (current === undefined) throw new Error('no vehicle');

    const invalid: typeof current = {
      ...current,
      familySeenMs: { ...current.familySeenMs, ATTITUDE: Number.NaN },
    };

    expect(isFamilyFresh(invalid, 'ATTITUDE', T0)).toBe(false);
  });
});

describe('source-coherent accumulation', () => {
  const global = telemetryEvent(
    1,
    1,
    {
      case: 'globalPosition',
      value: create(GlobalPositionSchema, {
        latDeg: 37.7,
        lonDeg: -122.4,
        altRelativeM: 25,
        altMslM: 130,
      }),
    },
    T0,
  );
  const gps = telemetryEvent(
    1,
    1,
    {
      case: 'gpsRaw',
      value: create(GpsRawSchema, { latDeg: 10, lonDeg: 20, altMslM: 500 }),
    },
    T0 + 1,
  );
  const system = telemetryEvent(
    1,
    1,
    {
      case: 'systemStatus',
      value: create(SystemStatusSchema, {
        voltageBatteryMv: 12_000,
        currentBatteryCa: 100,
        batteryRemainingPct: 50,
      }),
    },
    T0,
  );
  const battery = telemetryEvent(
    1,
    1,
    {
      case: 'batteryStatus',
      value: create(BatteryStatusSchema, {
        cellVoltagesMv: [4000, 4000],
        currentBatteryCa: 200,
        batteryRemainingPct: 80,
      }),
    },
    T0 + 1,
  );

  it.each([
    ['global then GPS', [global, gps]],
    ['GPS then global', [gps, global]],
  ])('keeps the resolved position within one family: %s', (_name, events) => {
    const view = reduce(...events).vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(resolvePosition(view.sample)).toEqual({
      latDeg: 37.7,
      lonDeg: -122.4,
      altM: 25,
      altRef: 'RELATIVE',
      source: 'GLOBAL_POSITION_INT',
    });
  });

  it.each([
    ['system then battery', [system, battery]],
    ['battery then system', [battery, system]],
  ])('keeps the resolved power fields within one family: %s', (_name, events) => {
    const view = reduce(...events).vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(resolveBattery(view.sample)).toEqual({
      voltageV: 8,
      currentA: 2,
      remainingPct: 80,
      source: 'BATTERY_STATUS',
    });
  });

  it('does not mix a SYS_STATUS voltage into a BATTERY_STATUS reading', () => {
    const emptyBattery = telemetryEvent(
      1,
      1,
      {
        case: 'batteryStatus',
        value: create(BatteryStatusSchema, {
          cellVoltagesMv: [],
          currentBatteryCa: 200,
          batteryRemainingPct: 80,
        }),
      },
      T0 + 1,
    );
    const view = reduce(system, emptyBattery).vehicles[vehicleKey(1, 1)];
    if (view === undefined) throw new Error('no vehicle');

    expect(resolveBattery(view.sample)).toEqual({
      voltageV: null,
      currentA: 2,
      remainingPct: 80,
      source: 'BATTERY_STATUS',
    });
  });
});

describe('isActive', () => {
  it('is false only for a lost vehicle', () => {
    const discovered = reduce(fleetEvent(FleetEventType.VEHICLE_DISCOVERED, 1, 1));
    const lost = reduce(fleetEvent(FleetEventType.VEHICLE_LOST, 1, 1));

    const discoveredView = discovered.vehicles[vehicleKey(1, 1)];
    const lostView = lost.vehicles[vehicleKey(1, 1)];

    if (discoveredView === undefined || lostView === undefined) {
      throw new Error('no vehicle in state');
    }

    expect(isActive(discoveredView)).toBe(true);
    expect(isActive(lostView)).toBe(false);
  });
});
