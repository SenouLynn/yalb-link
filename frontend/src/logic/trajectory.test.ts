import { describe, expect, it } from 'vitest';

import { STALL_SPEED_MPS, trajectoryFixtures } from './__fixtures__/trajectory.fixtures';
import {
  HORIZON_S,
  TRAJECTORY_POINTS,
  type EnuOffset,
  resolvePredictiveTrajectory,
} from './trajectory';
import { baseSample } from './testing';

/**
 * The final predicted point.
 *
 * Indexed rather than `Array.prototype.at`, which is ES2022 while
 * tsconfig.app.json targets ES2020.
 */
function lastPoint(points: EnuOffset[]): EnuOffset | undefined {
  return points.length === 0 ? undefined : points[points.length - 1];
}

/** Straight-line distance from the vehicle to the last predicted point. */
function forwardProgress(points: EnuOffset[]): number {
  const last = lastPoint(points);

  return last === undefined ? 0 : Math.hypot(last.northM, last.eastM);
}

describe('resolvePredictiveTrajectory', () => {
  it.each(trajectoryFixtures)('$name', ({ input, stallSpeedMps, expectMotion }) => {
    const points = resolvePredictiveTrajectory(baseSample(input), stallSpeedMps);

    if (expectMotion) {
      expect(points).toHaveLength(TRAJECTORY_POINTS);
      expect(forwardProgress(points)).toBeGreaterThan(0);
    } else {
      expect(forwardProgress(points)).toBe(0);
    }
  });

  it('gates on flight regime, not vehicle type', () => {
    // A hovering VTOL is a fixed-wing airframe with no airspeed reading. It
    // must still predict — this is the regression that keying the stall gate
    // on MAV_TYPE introduces.
    const hoveringVtol = resolvePredictiveTrajectory(
      baseSample({ groundspeedMps: 10, headingDeg: 90, rollRad: 0.1 }),
      STALL_SPEED_MPS,
    );

    expect(forwardProgress(hoveringVtol)).toBeGreaterThan(0);

    // The same speed *with* an airspeed reading below stall does collapse.
    const stalled = resolvePredictiveTrajectory(
      baseSample({ airspeedMps: 10, groundspeedMps: 10, headingDeg: 90, rollRad: 0.1 }),
      STALL_SPEED_MPS,
    );

    expect(stalled).toHaveLength(0);
  });

  it('projects straight-line distance as speed times horizon', () => {
    const points = resolvePredictiveTrajectory(
      baseSample({ groundspeedMps: 10, headingDeg: 0, rollRad: 0 }),
      STALL_SPEED_MPS,
    );

    // Wings level on a heading of north: 10 m/s over 5 s is 50 m due north.
    expect(lastPoint(points)?.northM).toBeCloseTo(10 * HORIZON_S, 6);
    expect(lastPoint(points)?.eastM).toBeCloseTo(0, 6);
  });

  it('curves the track when banked', () => {
    const banked = resolvePredictiveTrajectory(
      baseSample({ groundspeedMps: 20, headingDeg: 0, rollRad: 0.5 }),
      STALL_SPEED_MPS,
    );

    // A right bank starting northbound must deflect the path east.
    expect(lastPoint(banked)?.eastM).toBeGreaterThan(0);
  });

  it('never returns NaN, including at the pitch singularity', () => {
    const cases = [
      ...trajectoryFixtures.map((f) => f.input),
      // cos(pitch) is 0 here, which would divide by zero in the body-rate
      // transform.
      { groundspeedMps: 10, headingDeg: 0, rollRad: 0.2, pitchRad: Math.PI / 2,
        pitchspeedRadS: 0.1, yawspeedRadS: 0.1 },
      // 90 degrees of bank makes tan blow up in the fallback path.
      { groundspeedMps: 10, headingDeg: 0, rollRad: Math.PI / 2 },
    ];

    for (const input of cases) {
      for (const point of resolvePredictiveTrajectory(baseSample(input), STALL_SPEED_MPS)) {
        expect(Number.isFinite(point.northM)).toBe(true);
        expect(Number.isFinite(point.eastM)).toBe(true);
      }
    }
  });
});
