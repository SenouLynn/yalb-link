/** Adapts generated telemetry events to the resolver-facing sample shape. */

import type { MavAutopilot } from '@/gen/gcs/v1/types_pb';
import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';

/** A partial, flat projection in SI units and degrees, except documented wire fields. */
export interface TelemetrySample {
  // Attitude — radians
  rollRad?: number | undefined;
  pitchRad?: number | undefined;
  yawRad?: number | undefined;
  rollspeedRadS?: number | undefined;
  pitchspeedRadS?: number | undefined;
  yawspeedRadS?: number | undefined;

  // Position — kept per family so asynchronous messages cannot create a
  // coordinate tuple assembled from different observations.
  globalLatDeg?: number | undefined;
  globalLonDeg?: number | undefined;
  globalAltMslM?: number | undefined;
  /** Above home/takeoff. Only GLOBAL_POSITION_INT carries this. */
  globalAltRelativeM?: number | undefined;
  gpsLatDeg?: number | undefined;
  gpsLonDeg?: number | undefined;
  /** GPS_RAW_INT altitude is above mean sea level. */
  gpsAltMslM?: number | undefined;

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
  batteryStatusCellVoltagesMv?: number[] | undefined;
  batteryStatusCurrentCa?: number | undefined;
  batteryStatusRemainingPct?: number | undefined;
  /** SYS_STATUS pack voltage, mV. 65535 means unknown. */
  systemStatusVoltageMv?: number | undefined;
  systemStatusCurrentCa?: number | undefined;
  systemStatusRemainingPct?: number | undefined;

  ekfFlags?: number | undefined;
  missionCurrentSeq?: number | undefined;

  // Guidance — NAV_CONTROLLER_OUTPUT. Bearings arrive signed, -180..+180.
  navBearingDeg?: number | undefined;
  targetBearingDeg?: number | undefined;
  /** Distance to the active waypoint, metres. */
  wpDistM?: number | undefined;
  /** Altitude error, metres. Positive means the vehicle is below the target. */
  altErrorM?: number | undefined;
  /** Wire value: ArduPlane sends cm/s despite the legacy protobuf field name. */
  aspdErrorRaw?: number | undefined;
  /** Crosstrack error, metres: lateral offset from the commanded leg. */
  xtrackErrorM?: number | undefined;

  // Home — HOME_POSITION. The datum relative altitude is measured against.
  homeLatDeg?: number | undefined;
  homeLonDeg?: number | undefined;
  homeAltMslM?: number | undefined;

  // Link — RADIO_STATUS. Signal strengths are 0..254; 255 means unknown.
  radioRssi?: number | undefined;
  radioRemrssi?: number | undefined;
  radioNoise?: number | undefined;
  radioRemnoise?: number | undefined;
  /** Free transmit buffer space, per cent. Low means back-pressure. */
  radioTxbufPct?: number | undefined;
  radioRxerrors?: number | undefined;

  /** Identity fields merged from HeartbeatState by the vehicle accumulator. */
  autopilot?: MavAutopilot | undefined;
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
        globalLatDeg: payload.value.latDeg,
        globalLonDeg: payload.value.lonDeg,
        globalAltMslM: payload.value.altMslM,
        globalAltRelativeM: payload.value.altRelativeM,
        vxMs: payload.value.vxMS,
        vyMs: payload.value.vyMS,
        vzMs: payload.value.vzMS,
        hdgCdeg: payload.value.hdgCdeg,
      };

    case 'gpsRaw':
      return {
        sourceMessage: 'GPS_RAW_INT',
        receivedAtMs,
        gpsLatDeg: payload.value.latDeg,
        gpsLonDeg: payload.value.lonDeg,
        gpsAltMslM: payload.value.altMslM,
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
        climbMps: payload.value.climbMS,
      };

    case 'batteryStatus':
      return {
        sourceMessage: 'BATTERY_STATUS',
        receivedAtMs,
        batteryStatusCellVoltagesMv: payload.value.cellVoltagesMv,
        batteryStatusCurrentCa: payload.value.currentBatteryCa,
        batteryStatusRemainingPct: payload.value.batteryRemainingPct,
      };

    case 'systemStatus':
      return {
        sourceMessage: 'SYS_STATUS',
        receivedAtMs,
        systemStatusVoltageMv: payload.value.voltageBatteryMv,
        systemStatusCurrentCa: payload.value.currentBatteryCa,
        systemStatusRemainingPct: payload.value.batteryRemainingPct,
      };

    case 'missionCurrent':
      return {
        sourceMessage: 'MISSION_CURRENT',
        receivedAtMs,
        missionCurrentSeq: payload.value.seq,
      };

    case 'ekfStatusReport':
      return {
        sourceMessage: 'EKF_STATUS_REPORT',
        receivedAtMs,
        ekfFlags: payload.value.flags,
      };

    case 'navControllerOutput':
      // navRollDeg and navPitchDeg are the controller's commanded attitude, not
      // the vehicle's. Showing them beside the artificial horizon would invite
      // reading a demand as a measurement, so they are declined.
      return {
        sourceMessage: 'NAV_CONTROLLER_OUTPUT',
        receivedAtMs,
        navBearingDeg: payload.value.navBearingDeg,
        targetBearingDeg: payload.value.targetBearingDeg,
        wpDistM: payload.value.wpDistM,
        altErrorM: payload.value.altErrorM,
        aspdErrorRaw: payload.value.aspdErrorMS,
        xtrackErrorM: payload.value.xtrackErrorM,
      };

    case 'homePosition':
      // The local NED offset, attitude quaternion and approach vector are
      // declined: they describe the EKF origin and a landing heading, and this
      // display has nothing that consumes either. timeUsec is declined because
      // familySeenMs already carries when home was last heard.
      return {
        sourceMessage: 'HOME_POSITION',
        receivedAtMs,
        homeLatDeg: payload.value.latDeg,
        homeLonDeg: payload.value.lonDeg,
        homeAltMslM: payload.value.altMslM,
      };

    case 'radioStatus':
      // `fixed` counts packets error correction recovered. It is a radio
      // self-diagnostic rather than a link-quality reading an operator acts on,
      // so it is declined; rxerrors is kept because a rising count is actionable.
      return {
        sourceMessage: 'RADIO_STATUS',
        receivedAtMs,
        radioRssi: payload.value.rssi,
        radioRemrssi: payload.value.remrssi,
        radioNoise: payload.value.noise,
        radioRemnoise: payload.value.remnoise,
        radioTxbufPct: payload.value.txbufPct,
        radioRxerrors: payload.value.rxerrors,
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
    // STATUSTEXT is a sequence, not a latest-value-wins field, so it cannot be
    // projected into TelemetrySample at all. It gets its own log in T-016.
    case 'statusText':
      return 'STATUSTEXT';
    default:
      return 'UNKNOWN';
  }
}
