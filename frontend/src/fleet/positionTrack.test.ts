import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import { GlobalPositionSchema, GpsRawSchema, TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import { resolvePosition } from '@/logic/position';
import type { StreamEvent } from '@/stream/events';
import { fleetReducer, initialFleetState, TELEMETRY_TTL_MS, type FleetState } from './state';

const T0 = 1_789_476_250_000;
const LON = -122.4194003;

function fix(source: 'global' | 'gps', lat: number, observed: number, received = observed, sysId = 1): StreamEvent {
  const fields = { latDeg: lat, lonDeg: LON, observedAt: timestampFromMs(observed) };
  return {
    kind: 'telemetry', receivedAtMs: received,
    event: create(TelemetryEventSchema, {
      vehicleId: create(VehicleIdSchema, { systemId: sysId, componentId: 1 }),
      payload: source === 'global'
        ? { case: 'globalPosition', value: create(GlobalPositionSchema, { ...fields, altRelativeM: 20 }) }
        : { case: 'gpsRaw', value: create(GpsRawSchema, { ...fields, altMslM: 30 }) },
    }),
  };
}

function fold(events: StreamEvent[], start: FleetState = initialFleetState): FleetState {
  return events.reduce((state, event) => fleetReducer(state, { type: 'stream', event }), start);
}

function getView(state: FleetState) {
  const view = state.vehicles['1:1'];
  if (view === undefined) throw new Error('missing vehicle');
  return view;
}

function vehicle(events: StreamEvent[]) {
  return getView(fold(events));
}

describe('position source ordering in track and distance', () => {
  it('does not insert lagging GPS locations between fresh global updates', () => {
    // Rounded positions from T-019's northward SITL trace: the GPS sensor fix
    // trails the fused position despite arriving in a newer backend event.
    // Keep this regression self-contained; temporary recordings may be pruned.
    const events = [
      fix('global', 37.7753016, T0),
      fix('global', 37.7753106, T0 + 200),
      fix('gps', 37.7753043, T0 + 201),
      fix('global', 37.7753196, T0 + 400),
      fix('global', 37.7753286, T0 + 600),
    ];
    const actual = vehicle(events);
    const globalOnly = vehicle(events.filter((_, i) => i !== 2));
    expect(actual.track).toEqual(globalOnly.track);
    expect(actual.odometerM).toBeCloseTo(3.002, 2);
    expect(actual.odometerM).toBe(globalOnly.odometerM);
    expect(actual.sample.gpsLatDeg).toBe(37.7753043);
    expect(actual.familySeenMs['GPS_RAW_INT']).toBe(T0 + 201);
  });

  it('keeps GPS-only track and switches to global without double appending', () => {
    const view = vehicle([
      fix('gps', 37.775, T0), fix('gps', 37.77501, T0 + 1000),
      fix('global', 37.77502, T0 + 1100), fix('gps', 37.775015, T0 + 1200),
      fix('global', 37.77503, T0 + 1300),
    ]);
    expect(view.track.map(p => p.latDeg)).toEqual([37.775, 37.77501, 37.77502, 37.77503]);
    expect(view.odometerM).toBeCloseTo(3.336, 2);
  });

  it('prefers global at a shared timestamp without counting a zero-time source jump', () => {
    const gpsFirst = vehicle([
      fix('gps', 37.775, T0), fix('global', 37.77501, T0),
      fix('global', 37.77502, T0 + 100),
    ]);
    const globalFirst = vehicle([
      fix('global', 37.77501, T0), fix('gps', 37.775, T0),
      fix('global', 37.77502, T0 + 100),
    ]);
    expect(gpsFirst.track).toEqual(globalFirst.track);
    expect(gpsFirst.odometerM).toBeCloseTo(1.112, 2);
    expect(gpsFirst.odometerM).toBe(globalFirst.odometerM);
  });

  it('corrects the last leg when global replaces a same-time GPS fallback', () => {
    const view = vehicle([
      fix('gps', 37.775, T0), fix('gps', 37.77503, T0 + 1000),
      fix('global', 37.77502, T0 + 1000),
    ]);
    expect(view.track).toHaveLength(2);
    expect(view.odometerM).toBeCloseTo(2.224, 2);
  });

  it('falls back at the exact global TTL boundary and returns to global', () => {
    const expiry = T0 + TELEMETRY_TTL_MS;
    const view = vehicle([
      fix('global', 37.775, T0),
      fix('gps', 37.77501, expiry - 1),
      fix('gps', 37.77502, expiry),
      fix('gps', 37.77503, expiry + 1000),
      fix('global', 37.77504, expiry + 1100),
      fix('gps', 37.775035, expiry + 1200),
    ]);
    expect(view.track.map(p => p.latDeg)).toEqual([37.775, 37.77502, 37.77503, 37.77504]);
    expect(view.odometerM).toBeCloseTo(4.448, 2);
  });

  it('does not let invalid global coordinates suppress valid GPS fixes', () => {
    const view = vehicle([fix('global', Number.NaN, T0), fix('gps', 37.775, T0 + 100)]);
    expect(view.track).toHaveLength(1);
    expect(view.track[0]?.latDeg).toBe(37.775);
  });

  it('does not erase a fallback point when same-time global coordinates are invalid', () => {
    const view = vehicle([fix('gps', 37.775, T0), fix('global', Number.NaN, T0)]);
    expect(view.track).toEqual([{ latDeg: 37.775, lonDeg: LON, atMs: T0 }]);
    expect(view.odometerM).toBe(0);
  });

  it('rejects duplicate/older family observations without rewinding merged position', () => {
    const start = fold([fix('global', 37.775, T0), fix('global', 37.77502, T0 + 200)]);
    const next = fold([
      fix('global', 37.77501, T0 + 100, T0 + 300),
      fix('global', 37.77499, T0 + 200, T0 + 400),
    ], start);
    expect(next.vehicles['1:1']).toBe(start.vehicles['1:1']);
    expect(resolvePosition(getView(next).sample)?.latDeg).toBe(37.77502);
  });

  it('rejects late global recovery older than the latest accepted GPS fix', () => {
    const start = fold([fix('gps', 37.775, T0), fix('gps', 37.77502, T0 + 200)]);
    const next = fold([fix('global', 37.77501, T0 + 100, T0 + 300)], start);
    expect(next.vehicles['1:1']).toBe(start.vehicles['1:1']);
  });

  it('uses observation time and does not count stale bootstrap positions as new travel', () => {
    const now = T0 + 60_000;
    const view = vehicle([
      fix('global', 37.773, T0, now),
      fix('gps', 37.774, T0 + 1000, now),
      fix('global', 37.775, now - 100, now),
      fix('global', 37.77501, now + 100, now + 200),
    ]);
    expect(view.track).toHaveLength(2);
    expect(view.track[0]?.atMs).toBe(now - 100);
    expect(view.firstFixAtMs).toBe(now - 100);
    expect(view.odometerM).toBeCloseTo(1.112, 2);
  });

  it('keeps source preference and odometers independent per vehicle', () => {
    const state = fold([
      fix('global', 37.775, T0),
      fix('gps', 37.775, T0, T0, 2),
      fix('gps', 37.77501, T0 + 1000),
      fix('gps', 37.77501, T0 + 1000, T0 + 1000, 2),
    ]);
    expect(state.vehicles['1:1']?.track).toHaveLength(1);
    expect(state.vehicles['1:1']?.odometerM).toBe(0);
    expect(state.vehicles['2:1']?.track).toHaveLength(2);
    expect(state.vehicles['2:1']?.odometerM).toBeCloseTo(1.112, 2);
  });

  it('separates retained session displacement from a fresh replay/reset', () => {
    const prior = fold([fix('global', 37.776, T0)]);
    const nextFix = fix('global', 37.775, T0 + 10_000);
    const retained = getView(fold([nextFix], prior));
    const reset = getView(fold([{ kind: 'reset', receivedAtMs: T0 + 9000 }, nextFix], prior));
    expect(retained.odometerM).toBeCloseTo(111.195, 2);
    expect(retained.odometerM).toBeGreaterThan(111);
    expect(reset.odometerM).toBe(0);
    expect(reset.track).toHaveLength(1);
  });
});
