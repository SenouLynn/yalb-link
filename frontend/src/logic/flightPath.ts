/** Flight path vector with an explicit, unmixed climb-rate source. */

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
    // Zero ground speed correctly yields a vertical ±π/2 path.
    fpaRad: Math.atan2(climb.climbMps, groundSpeedMps),
    source: climb.source,
  };
}

/** Positive-up climb: VFR_HUD passes through; NED vz is negated. */
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

/** Uses NED course while moving and heading below the motion threshold. */
function resolveTrack(sample: TelemetrySample): number | null {
  if (isNum(sample.vxMs) && isNum(sample.vyMs)) {
    if (Math.hypot(sample.vxMs, sample.vyMs) >= MOVING_THRESHOLD_MPS) {
      // NED: x is north, y is east, so the bearing is atan2(east, north).
      return normaliseDeg(Math.atan2(sample.vyMs, sample.vxMs) * RAD_TO_DEG);
    }
  }

  return resolveHeading(sample)?.headingDeg ?? null;
}
