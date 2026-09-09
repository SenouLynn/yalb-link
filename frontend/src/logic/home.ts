/** The home datum: what "above home" altitude is measured from. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface HomeResult {
  latDeg: number;
  lonDeg: number;
  /** Home elevation above mean sea level, metres. */
  altMslM: number;
  source: 'HOME_POSITION';
}

/**
 * Resolves home from HOME_POSITION.
 *
 * Unlike the flight families this is a reference point rather than a
 * measurement: it changes when home is set, not continuously. The caller is
 * responsible for ageing it against a TTL that reflects that — see the home
 * reading in `ui/readings.ts`.
 */
export function resolveHome(sample: TelemetrySample): HomeResult | null {
  const { homeLatDeg, homeLonDeg, homeAltMslM } = sample;

  if (!isNum(homeLatDeg) || !isNum(homeLonDeg) || !isNum(homeAltMslM)) {
    return null;
  }

  // ArduPilot reports an all-zero position until home is set. Rendering it
  // would place home at Null Island and, worse, imply that relative altitude
  // has a datum when it does not. Only the exact pair is the sentinel: a real
  // vehicle can sit on a zero meridian or the equator, just not both.
  if (homeLatDeg === 0 && homeLonDeg === 0) {
    return null;
  }

  return {
    latDeg: homeLatDeg,
    lonDeg: homeLonDeg,
    altMslM: homeAltMslM,
    source: 'HOME_POSITION',
  };
}
