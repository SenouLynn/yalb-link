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

import { resolveAirspeed, type AirspeedResult } from '@/logic/airspeed';
import { resolveAttitude, type AttitudeResult } from '@/logic/attitude';
import { resolveBattery, type BatteryResult } from '@/logic/battery';
import { resolveFlightPath2d, type FlightPathResult } from '@/logic/flightPath';
import { ageMs } from '@/logic/freshness';
import { resolveGuidance, type GuidanceResult } from '@/logic/guidance';
import { resolveHeading, type HeadingResult } from '@/logic/heading';
import { resolveHome, type HomeResult } from '@/logic/home';
import { resolveLinkQuality, type LinkQualityResult } from '@/logic/link';
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
  /**
   * The TTL this reading was aged against.
   *
   * Carried on the reading rather than defaulted at each call site because not
   * every family perishes at the same rate — home does not perish at all on a
   * flight timescale. A consumer that assumed one global TTL would drain the
   * provenance bar to empty under a value its own state calls live.
   */
  ttlMs: number;
}

/** The reading nothing has been received for. */
export const UNAVAILABLE: Reading<never> = {
  state: 'unavailable',
  value: null,
  source: null,
  ageMs: null,
  ttlMs: TELEMETRY_TTL_MS,
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
    ttlMs,
  };
}

/** Every instrument reading for one vehicle at one instant. */
export interface FlightReadings {
  attitude: Reading<AttitudeResult>;
  heading: Reading<HeadingResult>;
  position: Reading<PositionResult>;
  flightPath: Reading<FlightPathResult>;
  airspeed: Reading<AirspeedResult>;
  battery: Reading<BatteryResult>;
  guidance: Reading<GuidanceResult>;
  home: Reading<HomeResult>;
  link: Reading<LinkQualityResult>;
}

/**
 * How long a home position stays showable.
 *
 * Home is the one reading here that is not a measurement. ArduPilot sends
 * HOME_POSITION when home is set, not on an interval — a 45 s Copter capture
 * contained exactly one — so under the telemetry TTL it would dash five
 * seconds into every flight and never come back. That would be a worse lie
 * than showing it: the datum has not stopped being true, and the altitude tape
 * beside it is still labelled "above home".
 *
 * An hour bounds the claim without contradicting it. Provenance still reports
 * the real age, so an operator reads when home was set rather than inferring
 * that it is current.
 */
export const HOME_TTL_MS = 60 * 60 * 1000;

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
    airspeed: age(resolveAirspeed(sample), view, nowMs, ttlMs),
    battery: age(resolveBattery(sample), view, nowMs, ttlMs),
    guidance: age(resolveGuidance(sample), view, nowMs, ttlMs),
    home: age(resolveHome(sample), view, nowMs, HOME_TTL_MS),
    link: age(resolveLinkQuality(sample), view, nowMs, ttlMs),
  };
}

/** Fraction of its own TTL a reading has used, clamped to 0..1. */
export function stalenessFraction<T>(
  reading: Reading<T>,
  ttlMs: number = reading.ttlMs,
): number {
  if (reading.ageMs === null || ttlMs <= 0) {
    return 1;
  }

  return Math.min(1, Math.max(0, reading.ageMs / ttlMs));
}
