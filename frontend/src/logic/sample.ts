/** Adapts generated telemetry events to the resolver-facing sample shape. */

import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';

/** A partial, flat projection of vehicle state in SI units and degrees. */
export interface TelemetrySample {
  // Attitude — radians
  rollRad?: number | undefined;
  pitchRad?: number | undefined;
  yawRad?: number | undefined;
  rollspeedRadS?: number | undefined;
  pitchspeedRadS?: number | undefined;
  yawspeedRadS?: number | undefined;

  // Position — degrees and metres, already normalised by the codec
  latDeg?: number | undefined;
  lonDeg?: number | undefined;
  /** Above mean sea level. */
  altMslM?: number | undefined;
  /** Above home/takeoff. Only GLOBAL_POSITION_INT carries this. */
  altRelativeM?: number | undefined;

  // Velocity — NED, m/s. vzMs is positive *down*.
  vxMs?: number | undefined;
  vyMs?: number | undefined;
  vzMs?: number | undefined;

  // VFR
  groundspeedMps?: number | undefined;
  airspeedMps?: number | undefined;
  /** VFR_HUD heading. int16 on the wire — may arrive negative. */
  headingDeg?: number | undefined;
  /** VFR_HUD climb rate, positive *up*. Already positive-up on the wire. */
  climbMps?: number | undefined;

  /** GLOBAL_POSITION_INT heading, centidegrees. 65535 means unknown. */
  hdgCdeg?: number | undefined;
  /** GPS_RAW_INT course over ground, centidegrees. 65535 means unknown. */
  cogCdeg?: number | undefined;
  /** GPS_RAW_INT ground speed, cm/s. */
  velCmS?: number | undefined;
  /** GPS_RAW_INT fix quality, as the generated `GpsFixType` enum value. */
  gpsFixType?: number | undefined;
  satellitesVisible?: number | undefined;

  // Power
  batteryCellVoltagesMv?: number[] | undefined;
  batteryCurrentCa?: number | undefined;
  batteryRemainingPct?: number | undefined;

  ekfFlags?: number | undefined;

  /** Identity fields merged from HeartbeatState by the vehicle accumulator. */
  vehicleType?: number | undefined;
  customMode?: number | undefined;
  systemStatus?: number | undefined;
  armed?: boolean | undefined;

  /** The MAVLink message this partial came from, e.g. `"ATTITUDE"`. */
  sourceMessage: string;
  receivedAtMs: number;
}

/** Projects one event family into a timestamped partial sample. */
export function sampleFromEvent(
  event: TelemetryEvent,
  receivedAtMs: number,
): Partial<TelemetrySample> & Pick<TelemetrySample, 'sourceMessage' | 'receivedAtMs'> {
  const payload = event.payload;

  switch (payload.case) {
    case 'attitude':
      return {
        sourceMessage: 'ATTITUDE',
        receivedAtMs,
        rollRad: payload.value.rollRad,
        pitchRad: payload.value.pitchRad,
        yawRad: payload.value.yawRad,
        rollspeedRadS: payload.value.rollspeedRadS,
        pitchspeedRadS: payload.value.pitchspeedRadS,
        yawspeedRadS: payload.value.yawspeedRadS,
      };

    case 'globalPosition':
      return {
        sourceMessage: 'GLOBAL_POSITION_INT',
        receivedAtMs,
        latDeg: payload.value.latDeg,
        lonDeg: payload.value.lonDeg,
        altMslM: payload.value.altMslM,
        altRelativeM: payload.value.altRelativeM,
        vxMs: payload.value.vxMS,
        vyMs: payload.value.vyMS,
        vzMs: payload.value.vzMS,
        hdgCdeg: payload.value.hdgCdeg,
      };

    case 'gpsRaw':
      return {
        sourceMessage: 'GPS_RAW_INT',
        receivedAtMs,
        latDeg: payload.value.latDeg,
        lonDeg: payload.value.lonDeg,
        altMslM: payload.value.altMslM,
        cogCdeg: payload.value.cogCdeg,
        velCmS: payload.value.velCmS,
        gpsFixType: payload.value.fixType,
        satellitesVisible: payload.value.satellitesVisible,
      };

    case 'vfrHud':
      return {
        sourceMessage: 'VFR_HUD',
        receivedAtMs,
        airspeedMps: payload.value.airspeedMS,
        groundspeedMps: payload.value.groundspeedMS,
        headingDeg: payload.value.headingDeg,
        altMslM: payload.value.altMslM,
        climbMps: payload.value.climbMS,
      };

    case 'batteryStatus':
      return {
        sourceMessage: 'BATTERY_STATUS',
        receivedAtMs,
        batteryCellVoltagesMv: payload.value.cellVoltagesMv,
        batteryCurrentCa: payload.value.currentBatteryCa,
        batteryRemainingPct: payload.value.batteryRemainingPct,
      };

    case 'systemStatus':
      return {
        sourceMessage: 'SYS_STATUS',
        receivedAtMs,
        batteryCurrentCa: payload.value.currentBatteryCa,
        batteryRemainingPct: payload.value.batteryRemainingPct,
      };

    case 'ekfStatusReport':
      return {
        sourceMessage: 'EKF_STATUS_REPORT',
        receivedAtMs,
        ekfFlags: payload.value.flags,
      };

    default:
      // Unprojected families still contribute source and freshness.
      return {
        sourceMessage: sourceMessageFor(payload.case),
        receivedAtMs,
      };
  }
}

/** Maps a oneof case name to its MAVLink message name. */
function sourceMessageFor(kind: TelemetryEvent['payload']['case']): string {
  switch (kind) {
    case 'navControllerOutput':
      return 'NAV_CONTROLLER_OUTPUT';
    case 'radioStatus':
      return 'RADIO_STATUS';
    case 'missionCurrent':
      return 'MISSION_CURRENT';
    case 'homePosition':
      return 'HOME_POSITION';
    case 'statusText':
      return 'STATUSTEXT';
    default:
      return 'UNKNOWN';
  }
}
