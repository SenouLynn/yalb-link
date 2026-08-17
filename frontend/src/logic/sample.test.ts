import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  AttitudeSchema,
  GlobalPositionSchema,
  GpsRawSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';

import { sampleFromEvent } from './sample';
import { FIXED_NOW_MS } from './testing';

describe('sampleFromEvent', () => {
  it('flattens an ATTITUDE event', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'attitude',
        value: create(AttitudeSchema, { rollRad: 0.1, pitchRad: -0.2, yawRad: 1.5 }),
      },
    });

    const sample = sampleFromEvent(event, FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('ATTITUDE');
    expect(sample.rollRad).toBeCloseTo(0.1, 6);
    expect(sample.pitchRad).toBeCloseTo(-0.2, 6);
    expect(sample.receivedAtMs).toBe(FIXED_NOW_MS);
  });

  it('carries GLOBAL_POSITION_INT through in SI units without re-scaling', () => {
    // The proto boundary already normalised these. A shim that divided again
    // would put position 1e7 too small and altitude 1000x too small while
    // every value stayed finite — passing a NaN gate and locking wrong numbers
    // into the fixture tables.
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'globalPosition',
        value: create(GlobalPositionSchema, {
          latDeg: 47.6062,
          lonDeg: -122.3321,
          altMslM: 120.5,
          altRelativeM: 50.25,
          vzMS: 1.5,
        }),
      },
    });

    const sample = sampleFromEvent(event, FIXED_NOW_MS);

    expect(sample.latDeg).toBeCloseTo(47.6062, 6);
    expect(sample.lonDeg).toBeCloseTo(-122.3321, 6);
    expect(sample.altRelativeM).toBeCloseTo(50.25, 6);
    expect(sample.vzMs).toBeCloseTo(1.5, 6);
  });

  it('keeps centidegree and cm/s fields in wire units under wire-unit names', () => {
    // hdg_cdeg and cog_cdeg keep their wire units all the way to the resolver
    // because the unknown sentinel (65535) is defined in those units.
    const gpi = sampleFromEvent(
      create(TelemetryEventSchema, {
        payload: { case: 'globalPosition', value: create(GlobalPositionSchema, { hdgCdeg: 65535 }) },
      }),
      FIXED_NOW_MS,
    );

    expect(gpi.hdgCdeg).toBe(65535);

    const gps = sampleFromEvent(
      create(TelemetryEventSchema, {
        payload: { case: 'gpsRaw', value: create(GpsRawSchema, { cogCdeg: 9000, velCmS: 1500 }) },
      }),
      FIXED_NOW_MS,
    );

    expect(gps.cogCdeg).toBe(9000);
    expect(gps.velCmS).toBe(1500);
  });

  it('names the source message for every populated family', () => {
    const vfr = sampleFromEvent(
      create(TelemetryEventSchema, {
        payload: { case: 'vfrHud', value: create(VfrHudSchema, { climbMS: 2.5 }) },
      }),
      FIXED_NOW_MS,
    );

    expect(vfr.sourceMessage).toBe('VFR_HUD');
    expect(vfr.climbMps).toBeCloseTo(2.5, 6);
  });

  it('returns a partial with a source even for an empty oneof', () => {
    const sample = sampleFromEvent(create(TelemetryEventSchema, {}), FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('UNKNOWN');
    expect(sample.receivedAtMs).toBe(FIXED_NOW_MS);
  });

  it('holds no state — one event populates only its own family', () => {
    const attitude = sampleFromEvent(
      create(TelemetryEventSchema, {
        payload: { case: 'attitude', value: create(AttitudeSchema, { rollRad: 0.1 }) },
      }),
      FIXED_NOW_MS,
    );

    // Position is absent, not zero. A resolver needing it correctly returns
    // null until the fold merges a position partial in.
    expect(attitude.latDeg).toBeUndefined();
  });
});
