/** Attitude projection. Source is always ATTITUDE — there is no fallback. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface AttitudeResult {
  rollDeg: number;
  pitchDeg: number;
  source: 'ATTITUDE';
}

const RAD_TO_DEG = 180 / Math.PI;

/**
 * Returns roll and pitch in degrees, or `null` when ATTITUDE has not arrived.
 *
 * Null is the honest answer for a missing sensor: an artificial horizon that
 * renders 0/0 for "no data" shows a level aircraft, which is a specific and
 * wrong claim rather than an absent one.
 */
export function resolveAttitude(sample: TelemetrySample): AttitudeResult | null {
  const { rollRad, pitchRad } = sample;

  if (!isNum(rollRad) || !isNum(pitchRad)) {
    return null;
  }

  return {
    rollDeg: rollRad * RAD_TO_DEG,
    pitchDeg: pitchRad * RAD_TO_DEG,
    source: 'ATTITUDE',
  };
}
