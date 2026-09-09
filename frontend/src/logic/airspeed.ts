/** Airspeed, kept separate from the flight path vector. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface AirspeedResult {
  airspeedMps: number;
  source: 'VFR_HUD';
}

/**
 * Resolves airspeed from VFR_HUD.
 *
 * Deliberately its own reading rather than a field on FlightPathResult. That
 * result's `source` names where the *climb rate* came from, which is
 * GLOBAL_POSITION_INT whenever VFR_HUD is absent — and provenance is looked up
 * by that one string. Folding airspeed in would let a reading label itself with
 * a family it did not come from, which is the specific failure the per-family
 * sample split exists to prevent.
 */
export function resolveAirspeed(sample: TelemetrySample): AirspeedResult | null {
  return isNum(sample.airspeedMps)
    ? { airspeedMps: sample.airspeedMps, source: 'VFR_HUD' }
    : null;
}
