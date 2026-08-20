/**
 * Turns accumulated vehicle state into readings the display can render.
 *
 * This is the one place that decides whether a value may be shown as a number.
 * The resolvers already return `null` for an absent sensor; what they cannot
 * know is that a value they resolved successfully came from a message that
 * stopped arriving four seconds ago. Both cases have to reach the instruments
 * as something other than a number, and centralising that here means an
 * instrument cannot forget to ask.
 */

import { resolveAttitude, type AttitudeResult } from '@/logic/attitude';
import { resolveBattery, type BatteryResult } from '@/logic/battery';
import { resolveFlightPath2d, type FlightPathResult } from '@/logic/flightPath';
import { ageMs } from '@/logic/freshness';
import { resolveHeading, type HeadingResult } from '@/logic/heading';
import { resolvePosition, type PositionResult } from '@/logic/position';
import { isFamilyFresh, TELEMETRY_TTL_MS, type VehicleView } from '@/fleet/state';

/**
 * Why a reading is or is not showable.
 *
 * `stale` is deliberately distinct from `unavailable` so provenance can report
 * what stopped and when. Neither state is eligible for numeric display.
 */
export type ReadingState = 'live' | 'stale' | 'unavailable';

export interface Reading<T> {
  state: ReadingState;
  /** Present only when the state is `live` or `stale`. */
  value: T | null;
  /** The MAVLink family the value came from, for provenance. */
  source: string | null;
  /** Milliseconds since that family last arrived. */
  ageMs: number | null;
}

/** The reading nothing has been received for. */
export const UNAVAILABLE: Reading<never> = {
  state: 'unavailable',
  value: null,
  source: null,
  ageMs: null,
};

/** Reports whether a reading is current enough for the display to render. */
export function hasDisplayValue<T>(reading: Reading<T>): reading is Reading<T> & { value: T } {
  return reading.state === 'live' && reading.value !== null;
}

/** Ages a resolved value against the family it came from. */
function age<T extends { source: string }>(
  resolved: T | null,
  view: VehicleView,
  nowMs: number,
  ttlMs: number,
): Reading<T> {
  if (resolved === null) {
    return UNAVAILABLE;
  }

  const seen = view.familySeenMs[resolved.source];

  // Resolved from a family with no receipt time: identity merged from a
  // heartbeat rather than a telemetry message. Treat it as unavailable rather
  // than inventing an age.
  if (seen === undefined) {
    return UNAVAILABLE;
  }

  return {
    state: isFamilyFresh(view, resolved.source, nowMs, ttlMs) ? 'live' : 'stale',
    value: resolved,
    source: resolved.source,
    ageMs: ageMs(seen, nowMs),
  };
}

/** Every instrument reading for one vehicle at one instant. */
export interface FlightReadings {
  attitude: Reading<AttitudeResult>;
  heading: Reading<HeadingResult>;
  position: Reading<PositionResult>;
  flightPath: Reading<FlightPathResult>;
  battery: Reading<BatteryResult>;
}

export function readFlight(
  view: VehicleView,
  nowMs: number,
  ttlMs: number = TELEMETRY_TTL_MS,
): FlightReadings {
  const { sample } = view;

  return {
    attitude: age(resolveAttitude(sample), view, nowMs, ttlMs),
    heading: age(resolveHeading(sample), view, nowMs, ttlMs),
    position: age(resolvePosition(sample), view, nowMs, ttlMs),
    flightPath: age(resolveFlightPath2d(sample), view, nowMs, ttlMs),
    battery: age(resolveBattery(sample), view, nowMs, ttlMs),
  };
}

/** Fraction of the TTL a reading has used, clamped to 0..1. */
export function stalenessFraction<T>(
  reading: Reading<T>,
  ttlMs: number = TELEMETRY_TTL_MS,
): number {
  if (reading.ageMs === null || ttlMs <= 0) {
    return 1;
  }

  return Math.min(1, Math.max(0, reading.ageMs / ttlMs));
}
