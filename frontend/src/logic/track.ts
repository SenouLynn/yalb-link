/** Accumulates flown positions as local ENU offsets. */

import type { TelemetrySample } from './sample';
import { resolvePosition } from './position';

/** A point in the local tangent plane, metres from the track origin. */
export interface EnuPoint {
  northM: number;
  eastM: number;
  altM: number;
  atMs: number;
}

/** The geodetic anchor the track is measured from. */
export interface TrackOrigin {
  latDeg: number;
  lonDeg: number;
}

/** Maximum retained points; 100 seconds at 5 Hz. */
export const TRACK_CAPACITY = 500;

/** Metres per degree of latitude. Spherical approximation. */
const METRES_PER_DEG = 111319.49;

const DEG_TO_RAD = Math.PI / 180;

/** Appends a local equirectangular point without mutating the prior track. */
export function accumulateTrack(
  prev: EnuPoint[],
  sample: TelemetrySample,
  origin: TrackOrigin,
): EnuPoint[] {
  const position = resolvePosition(sample);

  if (position === null) {
    return prev;
  }

  if (!Number.isFinite(origin.latDeg) || !Number.isFinite(origin.lonDeg)) {
    return prev;
  }

  const point: EnuPoint = {
    northM: (position.latDeg - origin.latDeg) * METRES_PER_DEG,
    // Longitude degrees shrink toward the poles.
    eastM:
      (position.lonDeg - origin.lonDeg) * METRES_PER_DEG * Math.cos(origin.latDeg * DEG_TO_RAD),
    altM: position.altM,
    atMs: sample.receivedAtMs,
  };

  const next = [...prev, point];

  return next.length > TRACK_CAPACITY ? next.slice(next.length - TRACK_CAPACITY) : next;
}
