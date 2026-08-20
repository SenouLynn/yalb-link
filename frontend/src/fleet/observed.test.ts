import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';

import {
  AttitudeSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import {
  fleetReducer,
  initialFleetState,
  isFamilyFresh,
  TELEMETRY_TTL_MS,
  vehicleKey,
} from '@/fleet/state';
import type { StreamEvent } from '@/stream/events';

import { observedAtMs, TELEMETRY_PAYLOAD_ONEOF } from './observed';

const T0 = 1_700_000_000_000;

describe('observedAtMs', () => {
  it('reads the stamp the backend wrote', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'attitude',
        value: create(AttitudeSchema, { observedAt: timestampFromMs(T0) }),
      },
    });

    expect(observedAtMs(event)).toBe(T0);
  });

  it('keeps millisecond precision', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'vfrHud',
        value: create(VfrHudSchema, { observedAt: timestampFromMs(T0 + 437) }),
      },
    });

    expect(observedAtMs(event)).toBe(T0 + 437);
  });

  it('returns null when no stamp was sent', () => {
    const event = create(TelemetryEventSchema, {
      payload: { case: 'attitude', value: create(AttitudeSchema, {}) },
    });

    expect(observedAtMs(event)).toBeNull();
  });

  it('returns null for an empty payload', () => {
    expect(observedAtMs(create(TelemetryEventSchema, {}))).toBeNull();
  });

  it('every telemetry payload in the contract declares observed_at', () => {
    // Guards the assumption this file rests on: if a family were added without
    // a stamp, its freshness would silently fall back to receipt time.
    if (TELEMETRY_PAYLOAD_ONEOF === undefined) {
      throw new Error('TelemetryEvent has no payload oneof');
    }

    const missing = TELEMETRY_PAYLOAD_ONEOF.fields
      .filter(
        (field) =>
          field.fieldKind !== 'message' ||
          !field.message.fields.some((inner) => inner.name === 'observed_at'),
      )
      .map((field) => field.name);

    expect(missing).toEqual([]);
  });
});

describe('freshness uses backend time, not arrival time', () => {
  /**
   * The bug this guards: the hub replays retained state on every reconnect. A
   * browser that stamped arrival would show telemetry the backend observed ten
   * minutes ago as if it had just landed.
   */
  it('treats replayed bootstrap state as old, not as newly arrived', () => {
    const observedLongAgo = T0 - 10 * 60_000;

    const bootstrap: StreamEvent = {
      kind: 'telemetry',
      // Received now...
      receivedAtMs: T0,
      event: create(TelemetryEventSchema, {
        vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
        payload: {
          case: 'attitude',
          // ...but observed ten minutes ago.
          value: create(AttitudeSchema, {
            rollRad: 0.2,
            observedAt: timestampFromMs(observedLongAgo),
          }),
        },
      }),
    };

    const state = fleetReducer(initialFleetState, { type: 'stream', event: bootstrap });
    const view = state.vehicles[vehicleKey(1, 1)];

    if (view === undefined) throw new Error('no vehicle in state');

    expect(view.familySeenMs['ATTITUDE']).toBe(observedLongAgo);
    expect(isFamilyFresh(view, 'ATTITUDE', T0)).toBe(false);
  });

  it('still counts genuinely current telemetry as fresh', () => {
    const live: StreamEvent = {
      kind: 'telemetry',
      receivedAtMs: T0,
      event: create(TelemetryEventSchema, {
        vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
        payload: {
          case: 'attitude',
          value: create(AttitudeSchema, { observedAt: timestampFromMs(T0 - 100) }),
        },
      }),
    };

    const state = fleetReducer(initialFleetState, { type: 'stream', event: live });
    const view = state.vehicles[vehicleKey(1, 1)];

    if (view === undefined) throw new Error('no vehicle in state');

    expect(isFamilyFresh(view, 'ATTITUDE', T0)).toBe(true);
    expect(isFamilyFresh(view, 'ATTITUDE', T0 + TELEMETRY_TTL_MS)).toBe(false);
  });

  it('falls back to receipt time when a payload carries no stamp', () => {
    const unstamped: StreamEvent = {
      kind: 'telemetry',
      receivedAtMs: T0,
      event: create(TelemetryEventSchema, {
        vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
        payload: { case: 'attitude', value: create(AttitudeSchema, {}) },
      }),
    };

    const state = fleetReducer(initialFleetState, { type: 'stream', event: unstamped });
    const view = state.vehicles[vehicleKey(1, 1)];

    if (view === undefined) throw new Error('no vehicle in state');

    expect(view.familySeenMs['ATTITUDE']).toBe(T0);
  });
});
