/**
 * Predictive trajectory — where the vehicle will be over the next few seconds,
 * under a constant-turn-rate-and-velocity (CTRV) model.
 */

import { isNum } from './finite';
import { resolveHeading } from './heading';
import type { TelemetrySample } from './sample';

/** A local tangent-plane offset from the vehicle's current position. */
export interface EnuOffset {
  northM: number;
  eastM: number;
}

/** A map coordinate. Kept free of renderer-specific types. */
export interface GeoCoordinate {
  latDeg: number;
  lonDeg: number;
}

/** Seconds of prediction. */
export const HORIZON_S = 5;

/** Points emitted across the horizon. */
export const TRAJECTORY_POINTS = 10;

const GRAVITY_MPS2 = 9.80665;
const DEG_TO_RAD = Math.PI / 180;
const RAD_TO_DEG = 180 / Math.PI;
const EARTH_RADIUS_M = 6_371_008.8;

/** Below this yaw rate the arc is indistinguishable from a straight line. */
const STRAIGHT_LINE_YAW_RATE = 1e-4;

/**
 * Projects a CTRV path. The vehicle-specific stall floor applies only when
 * airspeed is present; VTOL hover and vehicles without airspeed use ground
 * velocity. An empty result means the available state cannot support a path.
 */
export function resolvePredictiveTrajectory(
  sample: TelemetrySample,
  stallSpeedMps?: number,
): EnuOffset[] {
  const heading = resolveHeading(sample);

  if (heading === null) {
    return [];
  }

  const speed = resolveSpeed(sample, stallSpeedMps);

  if (speed === null) {
    return [];
  }

  const yawRateRadS = resolveYawRate(sample, speed);

  if (yawRateRadS === null) {
    return [];
  }

  return integrate(heading.headingDeg * DEG_TO_RAD, speed, yawRateRadS);
}

/**
 * Places a short local prediction on the map.
 *
 * This local tangent-plane approximation is intentionally limited to the
 * five-second horizon. At a pole longitude is undefined, so no path is
 * returned rather than inventing one.
 */
export function projectTrajectoryToGeo(
  origin: GeoCoordinate,
  offsets: EnuOffset[],
): GeoCoordinate[] {
  if (offsets.length === 0) {
    return [];
  }

  const originLatRad = origin.latDeg * DEG_TO_RAD;
  const longitudeScale = Math.cos(originLatRad);

  if (Math.abs(longitudeScale) < 1e-6) {
    return [];
  }

  return [
    origin,
    ...offsets.map((offset) => ({
      latDeg: origin.latDeg + (offset.northM / EARTH_RADIUS_M) * RAD_TO_DEG,
      lonDeg:
        origin.lonDeg + (offset.eastM / (EARTH_RADIUS_M * longitudeScale)) * RAD_TO_DEG,
    })),
  ];
}

/** Resolves propagation speed, applying the stall floor to measured airspeed. */
function resolveSpeed(sample: TelemetrySample, stallSpeedMps?: number): number | null {
  const { airspeedMps } = sample;

  if (isNum(airspeedMps) && isNum(stallSpeedMps)) {
    if (airspeedMps < stallSpeedMps) {
      return null;
    }

    return airspeedMps;
  }

  if (isNum(sample.groundspeedMps)) {
    return sample.groundspeedMps;
  }

  if (isNum(sample.vxMs) && isNum(sample.vyMs)) {
    return Math.hypot(sample.vxMs, sample.vyMs);
  }

  return null;
}

/**
 * Yaw rate in rad/s.
 *
 * Full CTRV body-rate transform when the gyro rates are present:
 * `ψ̇ = (sinφ·q + cosφ·r) / cosθ`. When they are not, the bank-angle
 * approximation `ψ̇ = g·tan(φ) / V` holds for coordinated flight.
 */
function resolveYawRate(sample: TelemetrySample, speedMps: number): number | null {
  const { rollRad, pitchRad, pitchspeedRadS, yawspeedRadS } = sample;

  if (isNum(rollRad) && isNum(pitchRad) && isNum(pitchspeedRadS) && isNum(yawspeedRadS)) {
    const cosPitch = Math.cos(pitchRad);

    // Fall back at the ±90° pitch singularity.
    if (Math.abs(cosPitch) > 1e-6) {
      const rate =
        (Math.sin(rollRad) * pitchspeedRadS + Math.cos(rollRad) * yawspeedRadS) / cosPitch;

      if (Number.isFinite(rate)) {
        return rate;
      }
    }
  }

  if (isNum(rollRad)) {
    // Undefined at zero speed — a stationary vehicle has no turn radius.
    if (speedMps < STRAIGHT_LINE_YAW_RATE) {
      return 0;
    }

    const rate = (GRAVITY_MPS2 * Math.tan(rollRad)) / speedMps;

    // tan blows up approaching 90° of bank.
    return Number.isFinite(rate) ? rate : 0;
  }

  // Heading and speed support a straight-line projection without attitude.
  return 0;
}

/** Walks the CTRV model forward over the horizon. */
function integrate(headingRad: number, speedMps: number, yawRateRadS: number): EnuOffset[] {
  const step = HORIZON_S / TRAJECTORY_POINTS;
  const points: EnuOffset[] = [];

  for (let i = 1; i <= TRAJECTORY_POINTS; i += 1) {
    const t = step * i;

    if (Math.abs(yawRateRadS) < STRAIGHT_LINE_YAW_RATE) {
      points.push({
        northM: speedMps * t * Math.cos(headingRad),
        eastM: speedMps * t * Math.sin(headingRad),
      });

      continue;
    }

    // Closed-form CTRV arc.
    const turned = headingRad + yawRateRadS * t;
    const radius = speedMps / yawRateRadS;

    points.push({
      northM: radius * (Math.sin(turned) - Math.sin(headingRad)),
      eastM: radius * (Math.cos(headingRad) - Math.cos(turned)),
    });
  }

  return points;
}
