/** Accumulates flown positions as geodetic points for a map renderer. */

import { resolvePosition } from './position';
import type { TelemetrySample } from './sample';
import { TRACK_CAPACITY } from './track';

export interface GeoPoint {
  latDeg: number;
  lonDeg: number;
  atMs: number;
}

/** Appends a position without mutating the prior track. */
export function accumulateGeoTrack(prev: GeoPoint[], sample: TelemetrySample): GeoPoint[] {
  const position = resolvePosition(sample);

  if (position === null) {
    return prev;
  }

  const next = [
    ...prev,
    { latDeg: position.latDeg, lonDeg: position.lonDeg, atMs: sample.receivedAtMs },
  ];

  return next.length > TRACK_CAPACITY ? next.slice(next.length - TRACK_CAPACITY) : next;
}
