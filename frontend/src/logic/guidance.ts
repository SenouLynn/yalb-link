/** Navigation controller state: where the vehicle is being sent, and the error. */

import { MavAutopilot, MavType } from '@/gen/gcs/v1/types_pb';

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

const PLANE_TYPES = new Set<number>([
  MavType.FIXED_WING,
  MavType.VTOL_TAILSITTER_DUOROTOR,
  MavType.VTOL_TAILSITTER_QUADROTOR,
  MavType.VTOL_TILTROTOR,
  MavType.VTOL_FIXEDROTOR,
  MavType.VTOL_TAILSITTER,
  MavType.VTOL_TILTWING,
]);

function airspeedErrorMps(sample: TelemetrySample, raw: number): number | null {
  if (sample.autopilot === undefined) return null;
  if (sample.autopilot !== MavAutopilot.ARDUPILOTMEGA) return raw;
  if (sample.vehicleType === undefined) return null;
  // ArduPlane 4.6.3 GCS_Mavlink.cpp sends airspeed_error * 100 (PR#7933).
  // This includes QuadPlane, but must not change Rover's standard m/s value.
  return PLANE_TYPES.has(sample.vehicleType) ? raw * 0.01 : raw;
}

export interface GuidanceResult {
  /** Heading the controller is commanding, degrees. */
  navBearingDeg: number;
  /** Bearing from the vehicle to the active waypoint, degrees. */
  targetBearingDeg: number;
  /** Distance to the active waypoint, metres. */
  wpDistM: number;
  /** Positive means below the commanded altitude. */
  altErrorM: number;
  /** Positive means slower than the commanded airspeed; null until identity is known. */
  aspdErrorMps: number | null;
  /** Lateral offset from the commanded leg, metres. */
  xtrackErrorM: number;
  source: 'NAV_CONTROLLER_OUTPUT';
}

/**
 * Resolves guidance from NAV_CONTROLLER_OUTPUT.
 *
 * Every field comes from the one message, so there is no preference order to
 * express here — the only question is whether the family has been heard. All
 * six are required together because they arrive together; a partial result
 * would mean the projection changed, not that the vehicle sent less.
 */
export function resolveGuidance(sample: TelemetrySample): GuidanceResult | null {
  const {
    navBearingDeg,
    targetBearingDeg,
    wpDistM,
    altErrorM,
    aspdErrorRaw,
    xtrackErrorM,
  } = sample;

  if (
    !isNum(navBearingDeg) ||
    !isNum(targetBearingDeg) ||
    !isNum(wpDistM) ||
    !isNum(altErrorM) ||
    !isNum(aspdErrorRaw) ||
    !isNum(xtrackErrorM)
  ) {
    return null;
  }

  return {
    navBearingDeg,
    targetBearingDeg,
    wpDistM,
    altErrorM,
    // Preserve the wire value for replay; normalize only after identity arrives.
    aspdErrorMps: airspeedErrorMps(sample, aspdErrorRaw),
    xtrackErrorM,
    source: 'NAV_CONTROLLER_OUTPUT',
  };
}
