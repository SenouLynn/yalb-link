/**
 * Heading resolution with a three-source fallback chain.
 *
 * Preference: `VFR_HUD.heading` → `ATTITUDE.yaw` → `GLOBAL_POSITION_INT.hdg`.
 * The winner is named in `source`, and `isFallback` says whether the primary
 * was available — an operator reading a heading off a degraded source should
 * be able to see that from the value alone.
 */

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

  // VFR_HUD.heading is int16_t on the wire, so it cannot carry the 65535
  // sentinel — testing for one here tests an input that never occurs. What it
  // can carry is a negative heading, which ArduPilot does emit, so the real
  // work on this path is wrapping rather than sentinel rejection.
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

  // GLOBAL_POSITION_INT.hdg is uint16 centidegrees, and this is where
  // UINT16_MAX-as-unknown actually lives. Reject it rather than reporting
  // 655.35 degrees.
  if (isNum(hdgCdeg) && hdgCdeg !== UNKNOWN_CDEG) {
    return {
      headingDeg: normaliseDeg(hdgCdeg / 100),
      source: 'GLOBAL_POSITION_INT',
      isFallback: true,
    };
  }

  return null;
}
