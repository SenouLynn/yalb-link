import { describe, expect, it } from 'vitest';

import { positionFixtures } from './__fixtures__/position.fixtures';
import { resolvePosition } from './position';
import { baseSample } from './testing';

describe('resolvePosition', () => {
  it.each(positionFixtures)('$name', ({ input, expected }) => {
    const actual = resolvePosition(baseSample(input));

    expect(actual).toEqual(expected);
  });

  it('reports the MSL datum on the GPS_RAW fallback, never RELATIVE', () => {
    // The specific failure altRef exists to prevent: at a field elevation of
    // ~120 m, mislabelling MSL as height-above-home puts the altitude tape out
    // by the entire field elevation.
    const actual = resolvePosition(
      baseSample({ latDeg: 47.6062, lonDeg: -122.3321, altMslM: 120.5 }),
    );

    expect(actual?.altRef).toBe('MSL');
    expect(actual?.source).toBe('GPS_RAW_INT');
  });

  it('uses plausible geodetic values, catching a 1e7 scaling error', () => {
    for (const { expected } of positionFixtures) {
      if (expected === null) {
        continue;
      }

      expect(Math.abs(expected.latDeg)).toBeLessThanOrEqual(90);
      expect(Math.abs(expected.lonDeg)).toBeLessThanOrEqual(180);
    }
  });

  it('never returns NaN', () => {
    for (const { input } of positionFixtures) {
      const actual = resolvePosition(baseSample(input));

      if (actual === null) {
        continue;
      }

      expect(Number.isNaN(actual.latDeg)).toBe(false);
      expect(Number.isNaN(actual.lonDeg)).toBe(false);
      expect(Number.isNaN(actual.altM)).toBe(false);
    }
  });
});
