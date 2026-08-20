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
  if (
    isNum(sample.globalLatDeg) &&
    isNum(sample.globalLonDeg) &&
    isNum(sample.globalAltRelativeM)
  ) {
    return {
      latDeg: sample.globalLatDeg,
      lonDeg: sample.globalLonDeg,
      altM: sample.globalAltRelativeM,
      altRef: 'RELATIVE',
      source: 'GLOBAL_POSITION_INT',
    };
  }

  if (isNum(sample.gpsLatDeg) && isNum(sample.gpsLonDeg) && isNum(sample.gpsAltMslM)) {
    return {
      latDeg: sample.gpsLatDeg,
      lonDeg: sample.gpsLonDeg,
      altM: sample.gpsAltMslM,
      altRef: 'MSL',
      source: 'GPS_RAW_INT',
    };
  }

  return null;
}
