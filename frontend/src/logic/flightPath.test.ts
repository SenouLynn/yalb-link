import { describe, expect, it } from 'vitest';

import { flightPathFixtures } from './__fixtures__/flightPath.fixtures';
import { resolveFlightPath2d } from './flightPath';
import { baseSample } from './testing';

describe('resolveFlightPath2d', () => {
  it.each(flightPathFixtures)('$name', ({ input, expected }) => {
    const actual = resolveFlightPath2d(baseSample(input));

    if (expected === null) {
      expect(actual).toBeNull();

      return;
    }

    expect(actual).not.toBeNull();
    expect(actual?.source).toBe(expected.source);
    expect(actual?.climbMps).toBeCloseTo(expected.climbMps, 6);
    expect(actual?.groundSpeedMps).toBeCloseTo(expected.groundSpeedMps, 6);
  });

  it('flips NED vz for climb but passes VFR_HUD through', () => {
    // Descending at 5 m/s in NED is vz = +5, which is climb = -5.
    const ned = resolveFlightPath2d(baseSample({ vzMs: 5, vxMs: 12, vyMs: 0, headingDeg: 0 }));

    expect(ned?.climbMps).toBeCloseTo(-5, 6);

    // The same physical descent from VFR_HUD arrives already positive-up.
    // heading is included because VFR_HUD carries climb, ground speed and
    // heading in one message — a sample with the first two and not the third
    // does not occur on the wire.
    const vfr = resolveFlightPath2d(
      baseSample({ climbMps: -5, groundspeedMps: 12, headingDeg: 0 }),
    );

    expect(vfr?.climbMps).toBeCloseTo(-5, 6);
  });

  it('derives fpa from climb over ground speed', () => {
    const actual = resolveFlightPath2d(
      baseSample({ climbMps: 10, groundspeedMps: 10, headingDeg: 0 }),
    );

    // Equal climb and ground speed is a 45 degree flight path angle.
    expect(actual?.fpaRad).toBeCloseTo(Math.PI / 4, 6);
  });

  it('never returns NaN', () => {
    for (const { input } of flightPathFixtures) {
      const actual = resolveFlightPath2d(baseSample(input));

      if (actual === null) {
        continue;
      }

      expect(Number.isNaN(actual.trackDeg)).toBe(false);
      expect(Number.isNaN(actual.climbMps)).toBe(false);
      expect(Number.isNaN(actual.groundSpeedMps)).toBe(false);
      expect(Number.isNaN(actual.fpaRad)).toBe(false);
    }
  });
});
