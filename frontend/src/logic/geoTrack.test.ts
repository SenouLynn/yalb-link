import { describe, expect, it } from 'vitest';

import { accumulateGeoTrack, type GeoPoint } from './geoTrack';
import { baseSample } from './testing';
import { TRACK_CAPACITY } from './track';

function sampleAt(latDeg: number, lonDeg: number, atMs = 1_700_000_000_000) {
  return baseSample({
    globalLatDeg: latDeg,
    globalLonDeg: lonDeg,
    globalAltRelativeM: 50,
    receivedAtMs: atMs,
  });
}

describe('accumulateGeoTrack', () => {
  it('accumulates points in order', () => {
    let track: GeoPoint[] = [];

    track = accumulateGeoTrack(track, sampleAt(47.6062, -122.3321, 1));
    track = accumulateGeoTrack(track, sampleAt(47.607, -122.3321, 2));
    track = accumulateGeoTrack(track, sampleAt(47.608, -122.3321, 3));

    expect(track).toEqual([
      { latDeg: 47.6062, lonDeg: -122.3321, atMs: 1 },
      { latDeg: 47.607, lonDeg: -122.3321, atMs: 2 },
      { latDeg: 47.608, lonDeg: -122.3321, atMs: 3 },
    ]);
  });

  it('does not mutate the input array', () => {
    const original: GeoPoint[] = [];
    const next = accumulateGeoTrack(original, sampleAt(47.607, -122.3321));

    expect(original).toHaveLength(0);
    expect(next).toHaveLength(1);
  });

  it('returns the previous array by reference when there is no fix', () => {
    const previous: GeoPoint[] = [];

    expect(accumulateGeoTrack(previous, baseSample({}))).toBe(previous);
  });

  it('caps at capacity, dropping the oldest points', () => {
    let track: GeoPoint[] = [];

    for (let i = 0; i < TRACK_CAPACITY + 50; i += 1) {
      track = accumulateGeoTrack(track, sampleAt(47 + i * 1e-5, -122, i));
    }

    expect(track).toHaveLength(TRACK_CAPACITY);
    expect(track[0]?.atMs).toBe(50);
    expect(track[track.length - 1]?.atMs).toBe(TRACK_CAPACITY + 49);
  });
});
