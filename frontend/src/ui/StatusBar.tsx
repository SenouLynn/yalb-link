/** Vehicle identity, armed state, mode, and the health of the data itself. */

import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { GpsFixType } from '@/gen/gcs/v1/types_pb';
import { isFamilyFresh, type VehicleView } from '@/fleet/state';
import type { StreamSource } from '@/stream/select';

import { NO_VALUE, sourceLabel } from './format';
import { Group, Row, type Tone } from './primitives';

export interface StatusRowsProps {
  view: VehicleView;
  nowMs: number;
  /** Whether the browser currently holds the backend stream. */
  connected: boolean;
  /** Where this page's data comes from. Never inferred from the data itself. */
  source: StreamSource;
}

/**
 * The link's own health, as rail rows.
 *
 * Separate from vehicle state because the two fail independently: a healthy
 * aircraft behind a dropped browser connection must not be reported as lost,
 * and a lost aircraft on a healthy connection must not look fine. This is the
 * one rail group the operator cannot hide — losing it is how you stop knowing
 * that the display is lying to you.
 */
export function LinkRows({ view, nowMs, connected, source }: StatusRowsProps) {
  const lost = view.lifecycle === FleetEventType.VEHICLE_LOST;

  return (
    <Group label="Link" reading={heartbeatReading(view, nowMs)}>
      <Row
        label="Source"
        value={sourceLabel(source, connected)}
        tone={sourceTone(source, connected)}
      />
      <Row
        label="Vehicle link"
        value={lost ? 'LOST' : 'HEARD'}
        tone={lost ? 'caution' : 'normal'}
      />
      <Row label="Target" value={view.key} />
    </Group>
  );
}

/** Armed state, mode, and the two estimators an operator checks before flying. */
export function StateRows({ view, nowMs }: StatusRowsProps) {
  const heartbeat = view.heartbeat;

  return (
    <Group label="Target state" reading={heartbeatReading(view, nowMs)}>
      <Row
        label="Armed"
        value={armedLabel(heartbeat?.armed)}
        tone={heartbeat === undefined ? 'dead' : heartbeat.armed ? 'caution' : 'normal'}
      />

      {/* Numeric: flight modes are firmware-specific and this display does not
          claim to decode them. Showing "4" is honest; showing "GUIDED" for the
          wrong airframe is not. */}
      <Row
        label="Mode"
        value={heartbeat === undefined ? NO_VALUE : String(heartbeat.customMode)}
        tone={heartbeat === undefined ? 'dead' : 'normal'}
      />

      <Row label="GPS" value={gpsLabel(view, nowMs)} tone={gpsTone(view, nowMs)} />
      <Row label="EKF" value={ekfLabel(view, nowMs)} tone={ekfTone(view, nowMs)} />
    </Group>
  );
}

function sourceTone(source: StreamSource, connected: boolean): Tone {
  if (source !== 'live') {
    return 'caution';
  }

  return connected ? 'normal' : 'caution';
}

function armedLabel(armed: boolean | undefined): string {
  if (armed === undefined) {
    return NO_VALUE;
  }

  return armed ? 'ARMED' : 'DISARMED';
}

/**
 * The GPS fix, named as the enum it is.
 *
 * The sample carries the generated enum's numeric value. Naming the type here
 * rather than at each use is what lets the switch below be exhaustive, instead
 * of a set of number comparisons that happen to line up with the enum today.
 */
function gpsFixOf(view: VehicleView): GpsFixType | undefined {
  return view.sample.gpsFixType;
}

function gpsLabel(view: VehicleView, nowMs: number): string {
  const gpsFixType = gpsFixOf(view);
  const { satellitesVisible } = view.sample;

  if (gpsFixType === undefined || !isFamilyFresh(view, 'GPS_RAW_INT', nowMs)) {
    return NO_VALUE;
  }

  const sats = satellitesVisible === undefined ? '' : ` · ${String(satellitesVisible)}`;

  return `${fixName(gpsFixType)}${sats}`;
}

function fixName(fix: GpsFixType): string {
  switch (fix) {
    case GpsFixType.GPS_FIX_TYPE_NO_GPS:
      return 'NO GPS';
    case GpsFixType.GPS_FIX_TYPE_NO_FIX:
      return 'NO FIX';
    case GpsFixType.GPS_FIX_TYPE_2D_FIX:
      return '2D';
    case GpsFixType.GPS_FIX_TYPE_3D_FIX:
      return '3D';
    case GpsFixType.GPS_FIX_TYPE_DGPS:
      return 'DGPS';
    case GpsFixType.GPS_FIX_TYPE_RTK_FLOAT:
      return 'RTK FLT';
    case GpsFixType.GPS_FIX_TYPE_RTK_FIXED:
      return 'RTK FIX';
    default:
      return `FIX ${String(fix)}`;
  }
}

function gpsTone(view: VehicleView, nowMs: number): Tone {
  const fix = gpsFixOf(view);

  if (fix === undefined || !isFamilyFresh(view, 'GPS_RAW_INT', nowMs)) {
    return 'dead';
  }

  return fix >= GpsFixType.GPS_FIX_TYPE_3D_FIX ? 'normal' : 'caution';
}

/** EKF_STATUS_FLAGS bits that decide whether the estimate is usable. */
const EKF_ATTITUDE = 1 << 0;
const EKF_UNINITIALIZED = 1 << 10;

function ekfLabel(view: VehicleView, nowMs: number): string {
  const flags = view.sample.ekfFlags;

  if (flags === undefined || !isFamilyFresh(view, 'EKF_STATUS_REPORT', nowMs)) {
    return NO_VALUE;
  }

  if ((flags & EKF_UNINITIALIZED) !== 0) {
    return 'INIT';
  }

  return (flags & EKF_ATTITUDE) !== 0 ? 'OK' : 'NO ATT';
}

function ekfTone(view: VehicleView, nowMs: number): Tone {
  const flags = view.sample.ekfFlags;

  if (flags === undefined || !isFamilyFresh(view, 'EKF_STATUS_REPORT', nowMs)) {
    return 'dead';
  }

  const healthy = (flags & EKF_UNINITIALIZED) === 0 && (flags & EKF_ATTITUDE) !== 0;

  return healthy ? 'normal' : 'caution';
}

function heartbeatReading(view: VehicleView, nowMs: number) {
  // Fleet events report changes, not every heartbeat (internal/vehicle/fold.go).
  // Age the retained observation; a LOST event must not reset it to "just heard".
  const observed = view.heartbeat?.observedAt;
  const seen = observed === undefined ? (view.heartbeat ? view.lastFleetAtMs : undefined)
    : Number(observed.seconds) * 1000 + observed.nanos / 1e6;
  const age = seen === undefined ? null : Math.max(0, nowMs - seen);
  return {
    state: age === null ? 'unavailable' as const : view.lifecycle === FleetEventType.VEHICLE_LOST ? 'stale' as const : 'live' as const,
    value: view.heartbeat ?? null, source: 'HEARTBEAT STATE', ageMs: age, ttlMs: Infinity,
  };
}
