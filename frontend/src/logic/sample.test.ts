import { create, fromJson } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  AttitudeSchema,
  GlobalPositionSchema,
  GpsRawSchema,
  HomePositionSchema,
  NavControllerOutputSchema,
  RadioStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';

import { resolveGuidance } from './guidance';
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

    expect(sample.globalLatDeg).toBeCloseTo(47.6062, 6);
    expect(sample.globalLonDeg).toBeCloseTo(-122.3321, 6);
    expect(sample.globalAltRelativeM).toBeCloseTo(50.25, 6);
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
    expect(attitude.globalLatDeg).toBeUndefined();
  });
});

describe('sampleFromEvent, families T-015 recovered from the default branch', () => {
  it('projects NAV_CONTROLLER_OUTPUT', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'navControllerOutput',
        value: create(NavControllerOutputSchema, {
          navBearingDeg: 91,
          targetBearingDeg: 94,
          wpDistM: 137,
          altErrorM: -2.4,
          aspdErrorMS: 0.6,
          xtrackErrorM: 3.1,
        }),
      },
    });

    const sample = sampleFromEvent(event, FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('NAV_CONTROLLER_OUTPUT');
    expect(sample.navBearingDeg).toBe(91);
    expect(sample.wpDistM).toBe(137);
    expect(sample.aspdErrorRaw).toBeCloseTo(0.6, 6);
  });

  it('projects HOME_POSITION', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'homePosition',
        value: create(HomePositionSchema, { latDeg: 37.7749, lonDeg: -122.4194, altMslM: 10.1 }),
      },
    });

    const sample = sampleFromEvent(event, FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('HOME_POSITION');
    expect(sample.homeLatDeg).toBeCloseTo(37.7749, 6);
    expect(sample.homeAltMslM).toBeCloseTo(10.1, 4);
  });

  it('projects RADIO_STATUS', () => {
    const event = create(TelemetryEventSchema, {
      payload: {
        case: 'radioStatus',
        value: create(RadioStatusSchema, { rssi: 190, remrssi: 185, txbufPct: 92, rxerrors: 7 }),
      },
    });

    const sample = sampleFromEvent(event, FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('RADIO_STATUS');
    expect(sample.radioRssi).toBe(190);
    expect(sample.radioTxbufPct).toBe(92);
  });

  it('resolves guidance from a recorded event whose zero fields JSON omitted', () => {
    // This is the exact shape GET /api/recordings/{id}/events returns: proto3
    // JSON drops zero-valued scalars, so a stationary vehicle's recorded
    // NAV_CONTROLLER_OUTPUT carries only navBearingDeg and observedAt. If the
    // parse did not restore schema defaults, every one of the six fields would
    // read undefined, the resolver would decline, and guidance would be blank
    // on replay while working perfectly live.
    const recorded = {
      vehicleId: { systemId: 1, componentId: 1 },
      navControllerOutput: {
        navRollDeg: -0.00011957914,
        navPitchDeg: 0.00010131222,
        navBearingDeg: 1,
        observedAt: '2026-09-09T17:29:29.691Z',
      },
    };

    const sample = sampleFromEvent(fromJson(TelemetryEventSchema, recorded), FIXED_NOW_MS);

    expect(sample.sourceMessage).toBe('NAV_CONTROLLER_OUTPUT');
    expect(sample.wpDistM).toBe(0);
    expect(resolveGuidance({ ...sample, sourceMessage: 'NAV_CONTROLLER_OUTPUT', receivedAtMs: FIXED_NOW_MS })).not.toBeNull();
  });
});
