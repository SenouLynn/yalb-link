/**
 * Flight path vector — track, ground speed, climb rate and flight path angle.
 *
 * The climb rate has two possible sources with opposite sign conventions, and
 * getting that wrong inverts the vertical needle. `source` names the winner;
 * sources are never mixed within one result.
 */

import { isNum } from './finite';
import { normaliseDeg, resolveHeading } from './heading';
import type { TelemetrySample } from './sample';

export interface FlightPathResult {
  trackDeg: number;
  groundSpeedMps: number;
  climbMps: number;
  /** Flight path angle, radians. Positive is climbing. */
  fpaRad: number;
  /** The message the *climb rate* came from. */
  source: 'VFR_HUD' | 'GLOBAL_POSITION_INT';
}

const RAD_TO_DEG = 180 / Math.PI;

/** Below this speed the velocity vector is noise rather than a track. */
const MOVING_THRESHOLD_MPS = 0.5;

export function resolveFlightPath2d(sample: TelemetrySample): FlightPathResult | null {
  const climb = resolveClimb(sample);

  if (climb === null) {
    return null;
  }

  const groundSpeedMps = resolveGroundSpeed(sample);

  if (groundSpeedMps === null) {
    return null;
  }

  const trackDeg = resolveTrack(sample);

  if (trackDeg === null) {
    return null;
  }

  return {
    trackDeg,
    groundSpeedMps,
    climbMps: climb.climbMps,
    // atan2 is safe at groundSpeedMps === 0: atan2(x, 0) is ±π/2, a vertical
    // flight path, which is the correct reading for a climbing hover.
    fpaRad: Math.atan2(climb.climbMps, groundSpeedMps),
    source: climb.source,
  };
}

/**
 * Climb rate, positive up.
 *
 * VFR_HUD.climb is already positive-up and passes through untouched. The
 * GLOBAL_POSITION_INT path carries NED velocity where vz is positive *down*,
 * so it is negated. Note there is no division: the proto boundary already
 * normalised cm/s to m/s, and dividing again here is the double-conversion
 * this codebase's unit rule exists to prevent.
 */
function resolveClimb(
  sample: TelemetrySample,
): { climbMps: number; source: FlightPathResult['source'] } | null {
  if (isNum(sample.climbMps)) {
    return { climbMps: sample.climbMps, source: 'VFR_HUD' };
  }

  if (isNum(sample.vzMs)) {
    return { climbMps: -sample.vzMs, source: 'GLOBAL_POSITION_INT' };
  }

  return null;
}

/** Ground speed, preferring VFR_HUD then the NED horizontal components. */
function resolveGroundSpeed(sample: TelemetrySample): number | null {
  if (isNum(sample.groundspeedMps)) {
    return sample.groundspeedMps;
  }

  if (isNum(sample.vxMs) && isNum(sample.vyMs)) {
    return Math.hypot(sample.vxMs, sample.vyMs);
  }

  return null;
}

/**
 * Track over ground.
 *
 * Course over ground from the NED velocity vector when the vehicle is moving,
 * since that is the direction actually travelled. Below the threshold the
 * velocity vector is noise, so heading stands in — a hovering copter has a
 * heading but no meaningful track.
 */
function resolveTrack(sample: TelemetrySample): number | null {
  if (isNum(sample.vxMs) && isNum(sample.vyMs)) {
    if (Math.hypot(sample.vxMs, sample.vyMs) >= MOVING_THRESHOLD_MPS) {
      // NED: x is north, y is east, so the bearing is atan2(east, north).
      return normaliseDeg(Math.atan2(sample.vyMs, sample.vxMs) * RAD_TO_DEG);
    }
  }

  return resolveHeading(sample)?.headingDeg ?? null;
}
