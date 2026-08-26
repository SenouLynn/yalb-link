/** Vehicle identity, armed state, mode, and the health of the data itself. */

import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { GpsFixType } from '@/gen/gcs/v1/types_pb';
import { isFamilyFresh, type VehicleView } from '@/fleet/state';
import type { StreamSource } from '@/stream/select';

import { NO_VALUE } from './format';

export interface StatusBarProps {
  view: VehicleView;
  nowMs: number;
  /** Whether the browser currently holds the backend stream. */
  connected: boolean;
  /** Where this page's data comes from. Never inferred from the data itself. */
  source: StreamSource;
}

/**
 * The top row.
 *
 * Link state and vehicle state are shown as separate chips because they fail
 * independently: a healthy aircraft behind a dropped browser connection must
 * not be reported as lost, and a lost aircraft on a healthy connection must
 * not look fine.
 */
export function StatusBar({ view, nowMs, connected, source }: StatusBarProps) {
  const heartbeat = view.heartbeat;
  const lost = view.lifecycle === FleetEventType.VEHICLE_LOST;

  return (
    <div className="panel status">
      <Chip label="Vehicle" value={view.key} tone="active" />

      <Chip
        label="State"
        value={armedLabel(heartbeat?.armed)}
        tone={heartbeat === undefined ? 'dead' : heartbeat.armed ? 'caution' : 'active'}
      />

      {/* Numeric: flight modes are firmware-specific and this display does not
          claim to decode them. Showing "4" is honest; showing "GUIDED" for the
          wrong airframe is not. */}
      <Chip
        label="Mode"
        value={heartbeat === undefined ? NO_VALUE : String(heartbeat.customMode)}
        tone={heartbeat === undefined ? 'dead' : 'active'}
      />

      <Chip label="GPS" value={gpsLabel(view, nowMs)} tone={gpsTone(view, nowMs)} />

      <Chip label="EKF" value={ekfLabel(view, nowMs)} tone={ekfTone(view, nowMs)} />

      <span className="status__spacer" />

      <Chip
        label="Vehicle link"
        value={lost ? 'LOST' : 'HEARD'}
        tone={lost ? 'caution' : 'active'}
      />

      <Chip label="Source" value={sourceLabel(source, connected)} tone={sourceTone(source, connected)} />
    </div>
  );
}

type Tone = 'active' | 'caution' | 'dead';

/**
 * What the display is showing, named plainly.
 *
 * Fixtures and recordings are both marked amber rather than green. Neither is
 * a flying aircraft, and an operator glancing at the chip has to be able to
 * tell that without reading the URL.
 */
function sourceLabel(source: StreamSource, connected: boolean): string {
  switch (source) {
    case 'mock':
      return 'MOCK';
    case 'replay':
      return 'REPLAY';
    default:
      return connected ? 'LIVE' : 'DISCONNECTED';
  }
}

function sourceTone(source: StreamSource, connected: boolean): Tone {
  if (source !== 'live') {
    return 'caution';
  }

  return connected ? 'active' : 'caution';
}

function Chip({ label, value, tone }: { label: string; value: string; tone: Tone }) {
  return (
    <span className={`chip chip--${tone}`}>
      <span className="label">{label}</span>
      <span>{value}</span>
    </span>
  );
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

  return fix >= GpsFixType.GPS_FIX_TYPE_3D_FIX ? 'active' : 'caution';
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

  return healthy ? 'active' : 'caution';
}
