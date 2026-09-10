import type { StreamSource } from '@/stream/select';
/**
 * Value formatting for the display.
 *
 * `NO_VALUE` is the only thing that appears where a number cannot. It is a
 * dash sequence rather than "0", "—", or an empty cell because an operator
 * scanning a panel has to be able to tell at a glance that the instrument is
 * not reporting, and a blank reads as a rendering bug rather than as an
 * absent sensor.
 */

/** Rendered wherever there is no number to show. */
export const NO_VALUE = '- - -';

/** Formats a number to fixed decimals, or NO_VALUE when there is none. */
export function num(value: number | null | undefined, digits = 1): string {
  return value === null || value === undefined || !Number.isFinite(value)
    ? NO_VALUE
    : value.toFixed(digits);
}

/** Formats a value that carries a sign the operator reads, like climb rate. */
export function signed(value: number | null | undefined, digits = 1): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return NO_VALUE;
  }

  return `${value >= 0 ? '+' : ''}${value.toFixed(digits)}`;
}

/** Formats a compass bearing as three padded degrees, the way a card reads. */
export function bearing(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return NO_VALUE;
  }

  return String(Math.round(((value % 360) + 360) % 360) % 360).padStart(3, '0');
}

/** Formats an age as a short duration: "0.4s", "12s", "3m". */
export function age(ms: number | null): string {
  if (ms === null || !Number.isFinite(ms) || ms < 0) {
    return NO_VALUE;
  }

  if (ms < 1000) {
    return `${(ms / 1000).toFixed(1)}s`;
  }

  if (ms < 60_000) {
    return `${String(Math.round(ms / 1000))}s`;
  }

  return `${String(Math.round(ms / 60_000))}m`;
}

/**
 * A geodetic pair on one line: `37.774908, -122.419394`.
 *
 * One row, not two. Latitude and longitude are never read apart — a latitude
 * alone locates nothing — so splitting them cost two row units and two labels
 * to say one thing, and put the two halves of a coordinate at different heights
 * on the shared value edge.
 */
export function latLon(
  latDeg: number | null | undefined,
  lonDeg: number | null | undefined,
  digits = 6,
): string {
  if (
    latDeg === null || latDeg === undefined || !Number.isFinite(latDeg) ||
    lonDeg === null || lonDeg === undefined || !Number.isFinite(lonDeg)
  ) {
    return NO_VALUE;
  }

  return `${latDeg.toFixed(digits)}, ${lonDeg.toFixed(digits)}`;
}

/**
 * A wire float, without its binary noise.
 *
 * `MISSION_ITEM.x` arrives as a float32 widened to double, so a waypoint
 * entered as 37.7769 comes back as 37.776900000000005 and renders eighteen
 * digits wide. Rounding to seven decimals and letting `Number` drop the
 * trailing zeros keeps every digit the autopilot actually holds — float32
 * carries about seven significant figures — while spending no width on the
 * ones it does not.
 */
export function tidy(value: number): string {
  return Number.isFinite(value) ? String(Number(value.toFixed(7))) : NO_VALUE;
}

/** Cardinal point for a bearing, for the heading card's letters. */
export function cardinal(deg: number): string | null {
  const points: Record<number, string> = { 0: 'N', 90: 'E', 180: 'S', 270: 'W' };

  return points[((Math.round(deg) % 360) + 360) % 360] ?? null;
}

/** Source identity comes from the selected transport, never inferred telemetry. */
export function sourceLabel(source: StreamSource, connected: boolean): string {
  switch (source) {
    case 'mock':
      return 'MOCK';
    case 'replay':
      return 'REPLAY';
    default:
      return connected ? 'LIVE' : 'DISCONNECTED';
  }
}
