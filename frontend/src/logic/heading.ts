/** Heading preference: VFR_HUD → ATTITUDE → GLOBAL_POSITION_INT. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface HeadingResult {
  headingDeg: number;
  source: 'VFR_HUD' | 'ATTITUDE' | 'GLOBAL_POSITION_INT';
  isFallback: boolean;
}

const RAD_TO_DEG = 180 / Math.PI;

/** UINT16_MAX, the "unknown" sentinel for centidegree heading fields. */
const UNKNOWN_CDEG = 65535;

/** Wraps any angle into [0, 360). */
export function normaliseDeg(deg: number): number {
  return ((deg % 360) + 360) % 360;
}

export function resolveHeading(sample: TelemetrySample): HeadingResult | null {
  const { headingDeg, yawRad, hdgCdeg } = sample;

  // VFR_HUD is signed and has no unknown sentinel.
  if (isNum(headingDeg)) {
    return {
      headingDeg: normaliseDeg(headingDeg),
      source: 'VFR_HUD',
      isFallback: false,
    };
  }

  if (isNum(yawRad)) {
    return {
      headingDeg: normaliseDeg(yawRad * RAD_TO_DEG),
      source: 'ATTITUDE',
      isFallback: true,
    };
  }

  // GLOBAL_POSITION_INT uses UINT16_MAX as unknown.
  if (isNum(hdgCdeg) && hdgCdeg !== UNKNOWN_CDEG) {
    return {
      headingDeg: normaliseDeg(hdgCdeg / 100),
      source: 'GLOBAL_POSITION_INT',
      isFallback: true,
    };
  }

  return null;
}
