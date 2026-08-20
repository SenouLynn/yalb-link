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
  SystemStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { GpsFixType, MavAutopilot, MavState, MavType } from '@/gen/gcs/v1/types_pb';
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

    if (i % 10 === 0) {
      frames.push(telemetryFrame(gpsRawAt, i));
      frames.push(telemetryFrame(systemStatusAt, i));
      frames.push(telemetryFrame(ekfAt, i));
    }
  }

  return frames;
}
