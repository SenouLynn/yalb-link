import { describe, expect, it } from 'vitest';

import { TRACK_CAPACITY, accumulateTrack, type EnuPoint, type TrackOrigin } from './track';
import { baseSample } from './testing';

const ORIGIN: TrackOrigin = { latDeg: 47.6062, lonDeg: -122.3321 };

function sampleAt(latDeg: number, lonDeg: number, altRelativeM = 50): ReturnType<typeof baseSample> {
  return baseSample({ latDeg, lonDeg, altRelativeM });
}

describe('accumulateTrack', () => {
  it('accumulates points in order', () => {
    let track: EnuPoint[] = [];

    track = accumulateTrack(track, sampleAt(47.6062, -122.3321), ORIGIN);
    track = accumulateTrack(track, sampleAt(47.607, -122.3321), ORIGIN);
    track = accumulateTrack(track, sampleAt(47.608, -122.3321), ORIGIN);

    expect(track).toHaveLength(3);
    expect(track[0]?.northM).toBeCloseTo(0, 6);
    // Each step north increases northM monotonically.
    expect(track[1]?.northM).toBeGreaterThan(track[0]?.northM ?? 0);
    expect(track[2]?.northM).toBeGreaterThan(track[1]?.northM ?? 0);
  });

  it('does not mutate the input array', () => {
    const original: EnuPoint[] = [];
    const next = accumulateTrack(original, sampleAt(47.607, -122.3321), ORIGIN);

    expect(original).toHaveLength(0);
    expect(next).toHaveLength(1);
  });

  it('returns the previous array by reference when there is no fix', () => {
    const previous: EnuPoint[] = [];
    // Identity, not just equality — callers use it to skip a re-render.
    expect(accumulateTrack(previous, baseSample({}), ORIGIN)).toBe(previous);
  });

  it('caps at capacity, dropping the oldest points', () => {
    let track: EnuPoint[] = [];

    for (let i = 0; i < TRACK_CAPACITY + 50; i += 1) {
      track = accumulateTrack(track, sampleAt(47.6062 + i * 1e-5, -122.3321), ORIGIN);
    }

    expect(track).toHaveLength(TRACK_CAPACITY);

    // The retained window is the newest one: the first point is the 51st
    // recorded, not the 1st.
    const fiftyFirst = 50 * 1e-5 * 111319.49;

    expect(track[0]?.northM).toBeCloseTo(fiftyFirst, 3);
  });

  it('projects ENU accurately at a 1 km offset', () => {
    // 1 km north is 1000 / 111319.49 degrees of latitude.
    const oneKmNorthDeg = 1000 / 111319.49;
    const track = accumulateTrack(
      [],
      sampleAt(ORIGIN.latDeg + oneKmNorthDeg, ORIGIN.lonDeg),
      ORIGIN,
    );

    expect(track[0]?.northM).toBeCloseTo(1000, 6);
    expect(track[0]?.eastM).toBeCloseTo(0, 6);
  });

  it('shrinks longitude degrees by latitude', () => {
    const oneDegEast = accumulateTrack([], sampleAt(ORIGIN.latDeg, ORIGIN.lonDeg + 1), ORIGIN);

    // At 47.6 degrees north, a degree of longitude is cos(47.6) of one at the
    // equator — about 75 km, not 111 km.
    const expected = 111319.49 * Math.cos((ORIGIN.latDeg * Math.PI) / 180);

    expect(oneDegEast[0]?.eastM).toBeCloseTo(expected, 3);
    expect(oneDegEast[0]?.eastM).toBeLessThan(111319.49);
  });

  it('never records NaN', () => {
    const track = accumulateTrack([], sampleAt(47.607, -122.333), ORIGIN);

    for (const point of track) {
      expect(Number.isFinite(point.northM)).toBe(true);
      expect(Number.isFinite(point.eastM)).toBe(true);
      expect(Number.isFinite(point.altM)).toBe(true);
    }
  });
});
