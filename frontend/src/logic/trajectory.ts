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

/** Seconds of prediction. */
export const HORIZON_S = 5;

/** Points emitted across the horizon. */
export const TRAJECTORY_POINTS = 10;

const GRAVITY_MPS2 = 9.80665;
const DEG_TO_RAD = Math.PI / 180;

/** Below this yaw rate the arc is indistinguishable from a straight line. */
const STRAIGHT_LINE_YAW_RATE = 1e-4;

/**
 * Projects the vehicle's path forward.
 *
 * `stallSpeedMps` is a parameter (ArduPilot's `ARSPD_FBW_MIN`), not a
 * constant, so Tier 7's parameter read can supply the vehicle's real value
 * without changing this signature.
 *
 * **The stall gate keys on flight regime, not vehicle type.** The floor
 * applies only when airspeed is actually present and below stall; otherwise
 * ground speed is used with no floor at all. Gating on MAV_TYPE instead would
 * blank the predicted track for a VTOL hovering or translating in multicopter
 * mode — the regime where an operator most wants to see one — because a
 * hovering VTOL is a fixed-wing airframe reporting near-zero airspeed.
 *
 * Returns an empty array rather than null when no prediction is possible, so
 * callers can render it directly without a null branch.
 */
export function resolvePredictiveTrajectory(
  sample: TelemetrySample,
  stallSpeedMps: number,
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
 * The speed to propagate, or null when the vehicle cannot sustain flight.
 *
 * Airspeed below stall means the aerodynamic model does not hold and any
 * forward projection would be fiction, so the prediction collapses. Absent
 * airspeed is not a stall — it is a vehicle that does not measure airspeed,
 * which is most copters — and ground speed is used unfloored.
 */
function resolveSpeed(sample: TelemetrySample, stallSpeedMps: number): number | null {
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

    // The transform has a genuine singularity at ±90° pitch. Rather than
    // emitting an infinite yaw rate, fall through to the bank approximation.
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

  // No attitude at all: assume straight-line flight rather than giving up.
  // Heading and speed alone still support a useful projection.
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

    // Closed-form CTRV arc. Exact, so accuracy does not decay with horizon
    // length the way Euler steps would.
    const turned = headingRad + yawRateRadS * t;
    const radius = speedMps / yawRateRadS;

    points.push({
      northM: radius * (Math.sin(turned) - Math.sin(headingRad)),
      eastM: radius * (Math.cos(headingRad) - Math.cos(turned)),
    });
  }

  return points;
}
