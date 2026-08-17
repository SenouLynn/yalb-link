import { describe, expect, it } from 'vitest';

import { resolveAttitude } from './attitude';
import { baseSample } from './testing';

describe('resolveAttitude', () => {
  it('converts radians to degrees', () => {
    const actual = resolveAttitude(baseSample({ rollRad: Math.PI / 6, pitchRad: -Math.PI / 12 }));

    expect(actual?.rollDeg).toBeCloseTo(30, 6);
    expect(actual?.pitchDeg).toBeCloseTo(-15, 6);
    expect(actual?.source).toBe('ATTITUDE');
  });

  it('returns null rather than a level attitude when ATTITUDE is absent', () => {
    // Rendering 0/0 for "no data" is a specific and wrong claim — it shows a
    // level aircraft — where null is an absent one.
    expect(resolveAttitude(baseSample({}))).toBeNull();
  });

  it('returns null when only one axis is present', () => {
    expect(resolveAttitude(baseSample({ rollRad: 0.5 }))).toBeNull();
    expect(resolveAttitude(baseSample({ pitchRad: 0.5 }))).toBeNull();
  });

  it('resolves a genuine zero attitude', () => {
    // Level flight is real data and must not be confused with missing data.
    const actual = resolveAttitude(baseSample({ rollRad: 0, pitchRad: 0 }));

    expect(actual).not.toBeNull();
    expect(actual?.rollDeg).toBe(0);
  });
});
