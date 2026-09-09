import { describe, expect, it } from 'vitest';

import { resolveGuidance } from './guidance';
import { MavAutopilot, MavType } from '@/gen/gcs/v1/types_pb';
import { baseSample } from './testing';

/** One complete NAV_CONTROLLER_OUTPUT projection. */
const navControllerSample = {
  navBearingDeg: 91,
  targetBearingDeg: 94,
  wpDistM: 137,
  altErrorM: -2.4,
  aspdErrorRaw: 60,
  autopilot: MavAutopilot.ARDUPILOTMEGA,
  vehicleType: MavType.FIXED_WING,
  xtrackErrorM: 3.1,
};

describe('resolveGuidance', () => {
  it.each([60, -125, 0])('converts ArduPilot wire error %s from cm/s to m/s', (raw) => {
    expect(resolveGuidance(baseSample({ ...navControllerSample, aspdErrorRaw: raw }))?.aspdErrorMps)
      .toBe(raw / 100);
  });

  it('preserves standard m/s for other autopilots', () => {
    expect(resolveGuidance(baseSample({ ...navControllerSample, autopilot: MavAutopilot.PX4, aspdErrorRaw: 0.6 }))?.aspdErrorMps)
      .toBe(0.6);
  });

  it.each([MavType.VTOL_TILTROTOR, MavType.VTOL_TAILSITTER_QUADROTOR])('normalizes QuadPlane type %s', (vehicleType) => {
    expect(resolveGuidance(baseSample({ ...navControllerSample, vehicleType }))?.aspdErrorMps).toBe(0.6);
  });

  it('does not apply the Plane scaling to ArduRover', () => {
    expect(resolveGuidance(baseSample({ ...navControllerSample,
      vehicleType: MavType.GROUND_ROVER, aspdErrorRaw: 0.6,
    }))?.aspdErrorMps).toBe(0.6);
  });

  it('withholds ArduPilot airspeed error when vehicle type is missing', () => {
    expect(resolveGuidance(baseSample({ ...navControllerSample, vehicleType: undefined }))?.aspdErrorMps).toBeNull();
  });

  it('withholds airspeed error until identity is available without hiding guidance', () => {
    const result = resolveGuidance(baseSample({ ...navControllerSample, autopilot: undefined }));
    expect(result?.aspdErrorMps).toBeNull();
    expect(result?.wpDistM).toBe(137);
  });

  it('resolves every field from NAV_CONTROLLER_OUTPUT', () => {
    expect(resolveGuidance(baseSample(navControllerSample))).toEqual({
      navBearingDeg: 91, targetBearingDeg: 94, wpDistM: 137,
      altErrorM: -2.4, aspdErrorMps: 0.6, xtrackErrorM: 3.1,
      source: 'NAV_CONTROLLER_OUTPUT',
    });
  });

  it('is unresolved before the family has been heard', () => {
    expect(resolveGuidance(baseSample())).toBeNull();
  });

  it('names the family it resolved from, so freshness can be looked up', () => {
    // `age()` in readings.ts keys familySeenMs by this exact string. A mismatch
    // makes every guidance readout permanently unavailable rather than stale,
    // which reads as "no such sensor" instead of "the vehicle stopped sending".
    expect(resolveGuidance(baseSample(navControllerSample))?.source).toBe(
      'NAV_CONTROLLER_OUTPUT',
    );
  });

  it('keeps a zero reading, which is on-track rather than absent', () => {
    // Perfect tracking sends zeros. Treating them as missing would blank the
    // readouts exactly when the vehicle is flying the leg correctly.
    const actual = resolveGuidance(
      baseSample({ ...navControllerSample, xtrackErrorM: 0, altErrorM: 0 }),
    );

    expect(actual?.xtrackErrorM).toBe(0);
    expect(actual?.altErrorM).toBe(0);
  });

  it('preserves the sign of the errors an operator reads directionally', () => {
    const actual = resolveGuidance(
      baseSample({ ...navControllerSample, altErrorM: -12.5, xtrackErrorM: -4 }),
    );

    expect(actual?.altErrorM).toBe(-12.5);
    expect(actual?.xtrackErrorM).toBe(-4);
  });

  it('is unresolved when a field is missing from the projection', () => {
    // The six arrive together on one message, so a partial sample means the
    // projection changed, not that the vehicle sent less. Resolving it anyway
    // would render a bearing of undefined as a plausible-looking number.
    expect(
      resolveGuidance(baseSample({ ...navControllerSample, navBearingDeg: undefined })),
    ).toBeNull();
  });
});
