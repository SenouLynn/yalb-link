/**
 * Deterministic playback fixtures.
 *
 * These are the same protobuf messages the backend sends, built by hand rather
 * than recorded, so the display can be developed and demonstrated with no
 * backend and no SITL. Every value is a pure function of the frame index:
 * running the fixture twice produces identical events, which is what makes it
 * usable as a test input as well as a demo.
 */

import { create } from '@bufbuild/protobuf';
import { timestampFromMs, type Timestamp } from '@bufbuild/protobuf/wkt';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';
import {
  AttitudeSchema,
  EkfStatusReportSchema,
  GlobalPositionSchema,
  GpsRawSchema,
  HomePositionSchema,
  MissionCurrentSchema,
  NavControllerOutputSchema,
  RadioStatusSchema,
  SystemStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { MOCK_MISSION_LENGTH } from '@/mission/fixtures';
import { GpsFixType, MavAutopilot, MavState, MavType, MissionState } from '@/gen/gcs/v1/types_pb';
import { HeartbeatStateSchema, VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

import type { StreamEvent } from './events';

/** The mock vehicle's identity: one autopilot, the ordinary single-vehicle case. */
export const MOCK_SYS_ID = 1;
export const MOCK_COMP_ID = 1;

/** Milliseconds between representative mock events; this is not a rate emulator. */
export const MOCK_FRAME_MS = 100;

/** One scheduled step of the fixture. */
export interface MockFrame {
  /** Delay before the *next* frame. */
  delayMs: number;
  build(receivedAtMs: number): StreamEvent;
}

function vehicleId() {
  return create(VehicleIdSchema, { systemId: MOCK_SYS_ID, componentId: MOCK_COMP_ID });
}

/** ArduCopter GUIDED. Numeric because the UI does not decode firmware modes. */
const GUIDED_CUSTOM_MODE = 4;

function heartbeatFrame(): MockFrame {
  return {
    delayMs: MOCK_FRAME_MS,
    build: (receivedAtMs) => ({
      kind: 'fleet',
      receivedAtMs,
      event: create(FleetEventSchema, {
        type: FleetEventType.VEHICLE_DISCOVERED,
        vehicleId: vehicleId(),
        heartbeat: create(HeartbeatStateSchema, {
          id: vehicleId(),
          type: MavType.QUADROTOR,
          autopilot: MavAutopilot.ARDUPILOTMEGA,
          systemStatus: MavState.ACTIVE,
          armed: true,
          customModeEnabled: true,
          customMode: GUIDED_CUSTOM_MODE,
          mavlinkVersion: 3,
        }),
      }),
    }),
  };
}

type TelemetryPayload = ReturnType<typeof create<typeof TelemetryEventSchema>>['payload'];

/**
 * Stamps observed_at the way the backend's fold does.
 *
 * Without it the mock would exercise the frontend's receipt-time fallback
 * rather than the path live telemetry actually takes, and the fixtures would
 * stop being a faithful stand-in for the backend.
 */
function stampObserved(event: TelemetryEvent, atMs: number): void {
  const value = event.payload.value;

  if (value !== undefined && typeof value === 'object' && 'observedAt' in value) {
    (value as { observedAt?: Timestamp | undefined }).observedAt = timestampFromMs(atMs);
  }
}

function telemetryFrame(
  payload: (index: number) => TelemetryPayload,
  index: number,
): MockFrame {
  return {
    delayMs: MOCK_FRAME_MS,
    build: (receivedAtMs) => {
      const event = create(TelemetryEventSchema, {
        vehicleId: vehicleId(),
        payload: payload(index),
      });

      stampObserved(event, receivedAtMs);

      return { kind: 'telemetry', receivedAtMs, event };
    },
  };
}

/** A slow banked turn: enough motion that a frozen display is obvious. */
function attitudeAt(index: number) {
  const phase = (index * Math.PI) / 40;

  return {
    case: 'attitude' as const,
    value: create(AttitudeSchema, {
      timeBootMs: index * MOCK_FRAME_MS,
      rollRad: Math.sin(phase) * 0.35,
      pitchRad: Math.sin(phase / 2) * 0.12,
      yawRad: (index * 0.02) % (2 * Math.PI),
    }),
  };
}

function vfrHudAt(index: number) {
  return {
    case: 'vfrHud' as const,
    value: create(VfrHudSchema, {
      airspeedMS: 6.2,
      groundspeedMS: 5.8 + Math.sin(index / 20) * 0.6,
      headingDeg: Math.round(((index * 1.15) % 360 + 360) % 360),
      throttlePct: 48,
      altMslM: 132.5,
      climbMS: Math.sin(index / 15) * 1.2,
    }),
  };
}

function globalPositionAt(index: number) {
  return {
    case: 'globalPosition' as const,
    value: create(GlobalPositionSchema, {
      timeBootMs: index * MOCK_FRAME_MS,
      latDeg: 37.7749 + index * 1e-6,
      lonDeg: -122.4194 + index * 8e-7,
      altMslM: 132.5,
      altRelativeM: 25 + Math.sin(index / 15) * 2,
      vxMS: 4.1,
      vyMS: 4.0,
      vzMS: -Math.sin(index / 15) * 1.2,
      hdgCdeg: Math.round(((index * 1.15) % 360 + 360) % 360) * 100,
    }),
  };
}

function gpsRawAt() {
  return {
    case: 'gpsRaw' as const,
    value: create(GpsRawSchema, {
      fixType: GpsFixType.GPS_FIX_TYPE_3D_FIX,
      latDeg: 37.7749,
      lonDeg: -122.4194,
      altMslM: 132.5,
      eph: 80,
      epv: 120,
      satellitesVisible: 14,
    }),
  };
}

function systemStatusAt(index: number) {
  return {
    case: 'systemStatus' as const,
    value: create(SystemStatusSchema, {
      loadDPct: 3200,
      voltageBatteryMv: 12_400 - index * 2,
      currentBatteryCa: 850,
      batteryRemainingPct: Math.max(0, 87 - Math.floor(index / 40)),
    }),
  };
}

/**
 * Walks the active item through the fixture mission.
 *
 * The panel marks an item active only while MISSION_CURRENT is inside the
 * telemetry freshness TTL, so the mock has to keep sending it exactly as a
 * vehicle does. A single frame at startup would go stale and the highlight
 * would silently vanish, which is the live defect this family had.
 */
function missionCurrentAt(index: number) {
  return {
    case: 'missionCurrent' as const,
    value: create(MissionCurrentSchema, {
      seq: Math.floor(index / 10) % MOCK_MISSION_LENGTH,
      total: MOCK_MISSION_LENGTH,
      missionState: MissionState.ACTIVE,
    }),
  };
}

/** Attitude solution present, filter initialised: the arm-permitting state. */
const EKF_HEALTHY_FLAGS = 0b0000_0000_1111_1111;

function ekfAt() {
  return {
    case: 'ekfStatusReport' as const,
    value: create(EkfStatusReportSchema, {
      flags: EKF_HEALTHY_FLAGS,
      velocityVariance: 0.12,
      posHorizVariance: 0.08,
      posVertVariance: 0.05,
      compassVariance: 0.1,
    }),
  };
}

/**
 * Guidance converging on the active waypoint.
 *
 * The distance walks down and the errors oscillate around zero so that a frozen
 * guidance panel is as obvious as a frozen attitude indicator. Errors cross zero
 * on purpose: a fixture that never renders a signed value would not exercise the
 * sign the operator reads.
 */
function navControllerAt(index: number) {
  const phase = index / 12;

  return {
    case: 'navControllerOutput' as const,
    value: create(NavControllerOutputSchema, {
      navRollDeg: Math.sin(phase) * 4,
      navPitchDeg: Math.sin(phase / 2) * 2,
      navBearingDeg: Math.round(((index * 1.15) % 360 + 360) % 360),
      targetBearingDeg: Math.round(((index * 1.15 + 3) % 360 + 360) % 360),
      wpDistM: Math.max(0, 240 - index * 2),
      altErrorM: Math.sin(phase) * 1.8,
      aspdErrorMS: Math.cos(phase) * 0.7,
      xtrackErrorM: Math.sin(phase / 1.5) * 2.4,
    }),
  };
}

/**
 * Home, matching the Compose SITL HOME_LOCATION.
 *
 * Constant on purpose: home does not move, and the mock would misrepresent the
 * family by animating it. The fixture sends it repeatedly only because the mock
 * has no other way to establish it; a real vehicle sends it once when home is set.
 */
function homePositionAt() {
  return {
    case: 'homePosition' as const,
    value: create(HomePositionSchema, {
      latDeg: 37.7749,
      lonDeg: -122.4194,
      altMslM: 10.1,
    }),
  };
}

/**
 * A SiK radio that is present and degrading.
 *
 * This family cannot arrive on the Compose stack — MAVLink there is UDP with no
 * radio in the path — so the fixture is the only place the link readouts can be
 * demonstrated at all. Free buffer space falls into the back-pressure range and
 * receive errors accumulate, which is the condition the readouts exist to show.
 */
function radioStatusAt(index: number) {
  return {
    case: 'radioStatus' as const,
    value: create(RadioStatusSchema, {
      rssi: 190 - Math.floor(index / 8),
      remrssi: 185 - Math.floor(index / 10),
      txbufPct: Math.max(2, 60 - index),
      noise: 38 + (index % 5),
      remnoise: 36 + (index % 4),
      rxerrors: Math.floor(index / 3),
      fixed: Math.floor(index / 12),
    }),
  };
}

/**
 * The fixture script.
 *
 * Fleet state first, matching the backend's bootstrap order, then a repeating
 * cycle that keeps fast flight state visibly moving while health families
 * update less often. Exact wire rates belong to the backend acquisition tests,
 * not to this deterministic UI demonstration.
 */
export function mockFrames(cycles = 60): MockFrame[] {
  const frames: MockFrame[] = [heartbeatFrame()];

  for (let i = 0; i < cycles; i++) {
    frames.push(telemetryFrame(attitudeAt, i));
    frames.push(telemetryFrame(vfrHudAt, i));
    frames.push(telemetryFrame(attitudeAt, i));
    frames.push(telemetryFrame(globalPositionAt, i));

    frames.push(telemetryFrame(missionCurrentAt, i));

    // Guidance changes as fast as the flight state it describes.
    frames.push(telemetryFrame(navControllerAt, i));

    // Every fifth cycle, not every tenth. The health families are read through
    // the same freshness TTL as everything else, so the gap between them has to
    // stay inside TELEMETRY_TTL_MS or the mock renders permanent dashes for
    // sensors it is actively sending. At ten the margin was already zero, and
    // adding a sixth frame to the cycle consumed it. mock.test.ts pins this.
    if (i % 5 === 0) {
      frames.push(telemetryFrame(gpsRawAt, i));
      frames.push(telemetryFrame(systemStatusAt, i));
      frames.push(telemetryFrame(ekfAt, i));
      frames.push(telemetryFrame(homePositionAt, i));
      frames.push(telemetryFrame(radioStatusAt, i));
    }
  }

  return frames;
}
