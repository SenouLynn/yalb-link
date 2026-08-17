/**
 * Flattens a `TelemetryEvent` oneof into a flat `TelemetrySample`.
 *
 * This file is the only place in the frontend that knows the proto envelope's
 * shape. Resolvers take a `TelemetrySample` and never import generated types.
 *
 * **Units are SI and degrees.** The Go codec normalises at the proto boundary,
 * so nothing here divides by 1e7, 1000 or 100 — with the two documented
 * exceptions the protos name explicitly (`hdgCdeg`, `cogCdeg` are
 * centidegrees; `velCmS` is cm/s). Those keep their wire units and their
 * wire-unit names all the way to the resolver that consumes them, because the
 * unknown-value sentinel is defined in wire units and checking it after a
 * conversion is how the sentinel gets lost.
 */

import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';

/**
 * A flat projection of vehicle sensor state.
 *
 * Fields are optional because `TelemetryEvent` is a oneof: a single event
 * carries exactly one family, so a single call to {@link sampleFromEvent}
 * populates only that family's fields. Resolvers that need fields spanning
 * families return `null` until a fold has merged enough partials.
 */
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

  /**
   * Vehicle identity and mode state.
   *
   * **Not populated by {@link sampleFromEvent}.** HEARTBEAT is not a
   * `TelemetryEvent` payload — it decodes to `HeartbeatState` and drives the
   * fleet fold instead. The Tier 4 per-vehicle fold merges those fields in
   * alongside the sensor partials. They live on this type so a resolver
   * needing vehicle class has somewhere to read it from; they are never set
   * from a telemetry event.
   */
  vehicleType?: number | undefined;
  customMode?: number | undefined;
  systemStatus?: number | undefined;
  armed?: boolean | undefined;

  /** The MAVLink message this partial came from, e.g. `"ATTITUDE"`. */
  sourceMessage: string;
  receivedAtMs: number;
}

/**
 * Projects one `TelemetryEvent` into a partial sample.
 *
 * Holds no state and returns only the fields the event's family carries.
 * Passing a single-event partial straight to a resolver commonly yields
 * `null` — that is correct behaviour, not a bug. The Tier 4 fold accumulates:
 *
 * ```ts
 * accumulated = { ...accumulated, ...sampleFromEvent(event, nowMs) };
 * ```
 *
 * `receivedAtMs` is a parameter rather than a `Date.now()` call so the
 * function stays pure and the fold stays testable against a fixed clock.
 */
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
      // Families with no sample projection yet (NAV_CONTROLLER_OUTPUT,
      // RADIO_STATUS, MISSION_CURRENT, HOME_POSITION, PID_TUNING,
      // NAMED_VALUE_*, STATUS_TEXT) and the empty oneof. They still carry a
      // source and a timestamp so freshness tracking sees the traffic.
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
    case 'pidTuning':
      return 'PID_TUNING';
    case 'namedValueFloat':
      return 'NAMED_VALUE_FLOAT';
    case 'namedValueInt':
      return 'NAMED_VALUE_INT';
    case 'statusText':
      return 'STATUSTEXT';
    default:
      return 'UNKNOWN';
  }
}
