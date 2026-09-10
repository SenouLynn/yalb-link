import { describe, expect, it } from 'vitest';

import type { GeoPoint } from './geoTrack';
import { greatCircleM, lastLegM } from './odometer';

const at = (latDeg: number, lonDeg: number, atMs = 0): GeoPoint => ({ latDeg, lonDeg, atMs });

describe('greatCircleM', () => {
  it('measures a degree of latitude as about 111 km', () => {
    expect(greatCircleM(at(0, 0), at(1, 0))).toBeCloseTo(111_195, -2);
  });

  it('is zero for a point against itself', () => {
    expect(greatCircleM(at(47.3977, 8.5456), at(47.3977, 8.5456))).toBe(0);
  });

  it('is symmetric', () => {
    const a = at(47.3977, 8.5456);
    const b = at(47.3988, 8.5471);
    expect(greatCircleM(a, b)).toBeCloseTo(greatCircleM(b, a), 9);
  });
});

describe('lastLegM', () => {
  it('measures the leg the newest fix closed', () => {
    const track = [at(0, 0), at(0.001, 0), at(0.002, 0)];
    expect(lastLegM(track)).toBeCloseTo(greatCircleM(at(0.001, 0), at(0.002, 0)), 6);
  });

  it('is zero for a single fix, which is a point and not a leg', () => {
    expect(lastLegM([at(0, 0)])).toBe(0);
    expect(lastLegM([])).toBe(0);
  });

  it('ignores sub-decimetre jitter so a parked vehicle does not accrue distance', () => {
    expect(lastLegM([at(47.3977, 8.5456), at(47.39770001, 8.5456)])).toBe(0);
  });

  it('drops a datum change, which is what an unfixed GPS frame at 0/0 produces', () => {
    // resolvePosition falls back to GPS_RAW_INT, and GPS_RAW_INT before a fix
    // reports Null Island. Summing that leg puts twelve thousand kilometres on
    // the odometer of an aircraft that has not left the pad.
    expect(lastLegM([at(0, 0), at(37.7749, -122.4194)])).toBe(0);
  });

  it('still measures once the ring is evicting, where the length stops growing', () => {
    // The case a length comparison gets wrong: the track is full, so an append
    // drops the head and the array stays 3 long. The leg is real regardless.
    const evicted = [at(0.001, 0), at(0.002, 0), at(0.003, 0)];
    expect(lastLegM(evicted)).toBeGreaterThan(100);
  });
});
