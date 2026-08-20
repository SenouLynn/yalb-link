/** Position resolution with an explicit altitude datum. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

/** `RELATIVE` is above home; `MSL` is above mean sea level. */
export type AltitudeRef = 'RELATIVE' | 'MSL';

export interface PositionResult {
  latDeg: number;
  lonDeg: number;
  altM: number;
  altRef: AltitudeRef;
  source: 'GLOBAL_POSITION_INT' | 'GPS_RAW_INT';
}

/** Resolves GLOBAL_POSITION_INT first, then GPS_RAW_INT, preserving the datum. */
export function resolvePosition(sample: TelemetrySample): PositionResult | null {
  const { latDeg, lonDeg, altRelativeM, altMslM } = sample;

  if (!isNum(latDeg) || !isNum(lonDeg)) {
    return null;
  }

  if (isNum(altRelativeM)) {
    return {
      latDeg,
      lonDeg,
      altM: altRelativeM,
      altRef: 'RELATIVE',
      source: 'GLOBAL_POSITION_INT',
    };
  }

  if (isNum(altMslM)) {
    return {
      latDeg,
      lonDeg,
      altM: altMslM,
      altRef: 'MSL',
      source: 'GPS_RAW_INT',
    };
  }

  return null;
}
