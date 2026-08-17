import { describe, expect, it } from 'vitest';

import { headingFixtures } from './__fixtures__/heading.fixtures';
import { resolveHeading } from './heading';
import { baseSample } from './testing';

describe('resolveHeading', () => {
  it.each(headingFixtures)('$name', ({ input, expected }) => {
    const actual = resolveHeading(baseSample(input));

    if (expected === null) {
      expect(actual).toBeNull();

      return;
    }

    expect(actual).not.toBeNull();
    expect(actual?.source).toBe(expected.source);
    expect(actual?.isFallback).toBe(expected.isFallback);
    expect(actual?.headingDeg).toBeCloseTo(expected.headingDeg, 6);
  });

  it('never returns NaN and always lands in [0, 360)', () => {
    for (const { input } of headingFixtures) {
      const actual = resolveHeading(baseSample(input));

      if (actual === null) {
        continue;
      }

      expect(Number.isNaN(actual.headingDeg)).toBe(false);
      expect(actual.headingDeg).toBeGreaterThanOrEqual(0);
      expect(actual.headingDeg).toBeLessThan(360);
    }
  });
});
