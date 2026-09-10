/**
 * Distance flown, accumulated as it arrives.
 *
 * Kept in the fold rather than measured off `VehicleView.track` because the
 * track is a fixed-capacity ring — 500 points, about 100 seconds at 5 Hz. A
 * distance summed over what the ring still holds would silently mean "the last
 * hundred seconds" while being labelled as the distance flown, and would *fall*
 * as the sortie went on. On a UAV that number is read against endurance, so a
 * value that shrinks under the operator is worse than no value at all.
 */

import type { GeoPoint } from './geoTrack';

/** Mean Earth radius, metres. The same spherical model `track.ts` uses. */
const EARTH_RADIUS_M = 6_371_008.8;

const DEG_TO_RAD = Math.PI / 180;

/** Below this a leg is GPS noise on a stationary aircraft. */
const MIN_LEG_M = 0.1;

/** Above this a leg is a change of datum rather than distance flown. */
const MAX_LEG_M = 10_000;

/**
 * Great-circle distance between two fixes, metres.
 *
 * Haversine rather than equirectangular: the increments summed here are metres
 * apart, where the two agree, but the same function frames a fleet spread over
 * a county and the error there is not worth the multiplication saved.
 */
export function greatCircleM(a: GeoPoint, b: GeoPoint): number {
  const lat1 = a.latDeg * DEG_TO_RAD;
  const lat2 = b.latDeg * DEG_TO_RAD;
  const dLat = lat2 - lat1;
  const dLon = (b.lonDeg - a.lonDeg) * DEG_TO_RAD;

  const h =
    Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLon / 2) ** 2;

  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(h)));
}

/**
 * The length of the track's final leg, metres.
 *
 * A pure function of one track. Whether that leg is *new* is a question only
 * the caller can answer, and it answers it by reference: `accumulateGeoTrack`
 * returns the array it was given when a frame carried no position, and a fresh
 * one when it appended. Deriving it from the length instead would look right
 * and then silently stop the odometer the moment the 500-point ring filled and
 * every append started evicting a point — the length stops changing there, and
 * that is precisely when a sortie is long enough for the number to matter.
 */
export function lastLegM(track: readonly GeoPoint[]): number {
  if (track.length < 2) {
    return 0;
  }

  const to = track[track.length - 1];
  const from = track[track.length - 2];

  if (from === undefined || to === undefined) {
    return 0;
  }

  const step = greatCircleM(from, to);

  // A fix that did not move is not a step, and GPS noise on a parked aircraft
  // is. Below a decimetre the increment is jitter, and summing jitter at 5 Hz
  // is how a stationary vehicle accrues a kilometre over a long bench session.
  if (step < MIN_LEG_M) {
    return 0;
  }

  /*
   * Above the ceiling the aircraft did not fly there.
   *
   * `resolvePosition` falls back from GLOBAL_POSITION_INT to GPS_RAW_INT, and a
   * GPS_RAW_INT frame that has not got a fix yet reports 0/0 — so the first two
   * points of a real flight can be Null Island and the runway, twelve thousand
   * kilometres apart. A replay seek and a GPS glitch produce the same shape. No
   * position feed on a UAV legitimately steps ten kilometres between fixes, so
   * a leg that long is a change of datum and not distance flown.
   */
  return step > MAX_LEG_M ? 0 : step;
}
