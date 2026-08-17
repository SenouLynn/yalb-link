/**
 * Position resolution with an explicit altitude datum.
 *
 * Primary is GLOBAL_POSITION_INT; fallback is GPS_RAW_INT.
 */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

/**
 * Which datum `altM` is measured against.
 *
 * `RELATIVE` is above home/takeoff; `MSL` is above mean sea level. They differ
 * by field elevation — routinely hundreds of metres — so this is not a label,
 * it is part of the value. A consumer that ignores it will fly an approach
 * against the wrong number.
 */
export type AltitudeRef = 'RELATIVE' | 'MSL';

export interface PositionResult {
  latDeg: number;
  lonDeg: number;
  altM: number;
  altRef: AltitudeRef;
  source: 'GLOBAL_POSITION_INT' | 'GPS_RAW_INT';
}

/**
 * Returns position with its altitude reference, or `null` when neither source
 * has produced a fix.
 *
 * The source is discriminated by field presence rather than by
 * `sample.sourceMessage`: samples are merged across families by the fold, so
 * by the time a resolver sees one, `sourceMessage` names only the most recent
 * event. `altRelativeM` is carried by GLOBAL_POSITION_INT alone, which makes
 * it the reliable discriminator.
 */
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

  // GPS_RAW_INT only ever gives MSL. Reporting it as relative altitude is the
  // exact failure `altRef` exists to prevent, so the datum changes with the
  // source.
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
