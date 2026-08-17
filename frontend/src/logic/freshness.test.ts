import { describe, expect, it } from 'vitest';

import { ageMs, isFresh } from './freshness';
import { FIXED_NOW_MS } from './testing';

const TTL_MS = 3000;

describe('isFresh', () => {
  it('is fresh below the TTL', () => {
    expect(isFresh(FIXED_NOW_MS - 2999, FIXED_NOW_MS, TTL_MS)).toBe(true);
  });

  it('is stale exactly at the TTL', () => {
    // The documented boundary: strictly less than TTL is fresh. Pinned here so
    // no component re-derives it the other way.
    expect(isFresh(FIXED_NOW_MS - TTL_MS, FIXED_NOW_MS, TTL_MS)).toBe(false);
  });

  it('is stale past the TTL', () => {
    expect(isFresh(FIXED_NOW_MS - 3001, FIXED_NOW_MS, TTL_MS)).toBe(false);
  });

  it('is never fresh with a non-positive TTL', () => {
    expect(isFresh(FIXED_NOW_MS, FIXED_NOW_MS, 0)).toBe(false);
    expect(isFresh(FIXED_NOW_MS, FIXED_NOW_MS, -1)).toBe(false);
  });
});

describe('ageMs', () => {
  it('measures elapsed milliseconds', () => {
    expect(ageMs(FIXED_NOW_MS - 1500, FIXED_NOW_MS)).toBe(1500);
  });

  it('clamps a backwards clock to zero rather than reporting a negative age', () => {
    expect(ageMs(FIXED_NOW_MS + 5000, FIXED_NOW_MS)).toBe(0);
  });

  it('treats a missing timestamp as infinitely old, never NaN', () => {
    expect(ageMs(Number.NaN, FIXED_NOW_MS)).toBe(Number.POSITIVE_INFINITY);
  });
});
