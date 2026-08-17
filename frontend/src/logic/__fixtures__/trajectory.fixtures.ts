import type { TelemetrySample } from '../sample';

export interface TrajectoryFixture {
  name: string;
  input: Partial<TelemetrySample>;
  stallSpeedMps: number;
  /** Whether the vehicle should show forward progress over the horizon. */
  expectMotion: boolean;
  note?: string;
}

/** ArduPilot's ARSPD_FBW_MIN default, the parameter the stall gate reads. */
export const STALL_SPEED_MPS = 14;

export const trajectoryFixtures: TrajectoryFixture[] = [
  {
    name: 'copter-with-angular-rates-full-ctrv',
    input: {
      groundspeedMps: 10,
      headingDeg: 0,
      rollRad: 0.3,
      pitchRad: 0.05,
      pitchspeedRadS: 0.01,
      yawspeedRadS: 0.15,
    },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: true,
    note: 'gyro rates present, so the body-rate transform is used',
  },
  {
    name: 'copter-without-angular-rates-bank-fallback',
    input: { groundspeedMps: 10, headingDeg: 0, rollRad: 0.3 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: true,
    note: 'no gyro, so the coordinated-flight bank approximation is used',
  },
  {
    name: 'fixed-wing-below-stall-collapses',
    input: { airspeedMps: 10, groundspeedMps: 10, headingDeg: 0, rollRad: 0 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: false,
    note: 'airspeed present and below stall — the aerodynamic model does not hold',
  },
  {
    name: 'fixed-wing-above-stall-predicts',
    input: { airspeedMps: 20, groundspeedMps: 20, headingDeg: 0, rollRad: 0 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: true,
  },
  {
    // The regression a vehicle-type gate causes. A hovering VTOL is a
    // fixed-wing airframe reporting no airspeed; keying the stall floor on
    // MAV_TYPE blanks its track in exactly the regime an operator watches most.
    name: 'vtol-hovering-no-airspeed-still-predicts',
    input: { groundspeedMps: 10, headingDeg: 90, rollRad: 0.1 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: true,
    note: 'no airspeed field at all — not a stall, just a vehicle not measuring it',
  },
  {
    name: 'stationary-hover-zero-yaw-rate',
    input: { groundspeedMps: 0, headingDeg: 0, rollRad: 0 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: false,
    note: 'zero speed produces points, all at the origin',
  },
  {
    name: 'no-heading-no-trajectory',
    input: { groundspeedMps: 10 },
    stallSpeedMps: STALL_SPEED_MPS,
    expectMotion: false,
  },
];
