import { describe, expect, it } from 'vitest';

import type { StreamEvent } from './events';
import { MOCK_COMP_ID, mockFrames } from './fixtures';
import { TELEMETRY_TTL_MS } from '@/fleet/state';
import { MockEventSource, type Cancel } from './mock';

const START_MS = 1_700_000_000_000;

/** A scheduler that runs on demand, so playback needs no timers. */
function manualScheduler() {
  const queue: { fn: () => void; delayMs: number }[] = [];

  const schedule = (fn: () => void, delayMs: number): Cancel => {
    queue.push({ fn, delayMs });

    return () => {
      const index = queue.findIndex((entry) => entry.fn === fn);
      if (index >= 0) queue.splice(index, 1);
    };
  };

  const runNext = (): boolean => {
    const next = queue.shift();
    if (next === undefined) return false;
    next.fn();

    return true;
  };

  return { schedule, runNext, pending: () => queue.length };
}

/** Plays a bounded number of steps and returns everything emitted. */
function play(steps: number, options: { loop?: boolean } = {}) {
  const clock = { ms: START_MS };
  const scheduler = manualScheduler();
  const events: StreamEvent[] = [];

  const source = new MockEventSource({
    now: () => clock.ms,
    schedule: scheduler.schedule,
    loop: options.loop ?? false,
  });

  const stop = source.start((event) => events.push(event));

  for (let i = 0; i < steps; i++) {
    clock.ms += 100;
    if (!scheduler.runNext()) break;
  }

  return { events, stop, scheduler };
}

describe('MockEventSource', () => {
  it('reports itself connected before emitting anything', () => {
    const { events, stop } = play(0);
    stop();

    expect(events[0]).toEqual({ kind: 'connection', connected: true, receivedAtMs: START_MS });
  });

  it('emits fleet state before any telemetry, matching the backend bootstrap', () => {
    const { events, stop } = play(5);
    stop();

    const discovered = new Set<number>();
    for (const event of events) {
      if (event.kind === 'fleet') discovered.add(event.event.vehicleId?.systemId ?? 0);
      if (event.kind === 'telemetry') expect(discovered.has(event.event.vehicleId?.systemId ?? 0)).toBe(true);
    }
    expect(discovered).toEqual(new Set([1, 2, 3]));
  });

  it('carries a known fleet identity on every event', () => {
    const { events, stop } = play(10);
    stop();

    for (const event of events) {
      if (event.kind === 'connection' || event.kind === 'reset') continue;

      expect([1, 2, 3]).toContain(event.event.vehicleId?.systemId);
      expect(event.event.vehicleId?.componentId).toBe(MOCK_COMP_ID);
    }
  });

  it('is deterministic: the same script twice produces the same values', () => {
    const first = play(40);
    first.stop();

    const second = play(40);
    second.stop();

    // Deep equality rather than JSON: uint64 fields deserialise to BigInt.
    expect(first.events).toEqual(second.events);
  });

  it('produces attitude that actually moves, so a frozen display is visible', () => {
    const { events, stop } = play(60);
    stop();

    const rolls = events.flatMap((event) =>
      event.kind === 'telemetry' && event.event.payload.case === 'attitude'
        ? [event.event.payload.value.rollRad]
        : [],
    );

    expect(rolls.length).toBeGreaterThan(2);
    expect(new Set(rolls).size).toBeGreaterThan(1);
  });

  it('stamps receipt from the injected clock, not the wall clock', () => {
    const { events, stop } = play(3);
    stop();

    const stamps = events.map((event) => event.receivedAtMs);

    expect(stamps[0]).toBe(START_MS);
    expect(stamps[stamps.length - 1]).toBeGreaterThan(START_MS);
  });

  it('stops scheduling once stopped', () => {
    const { stop, scheduler } = play(5);

    stop();

    expect(scheduler.runNext()).toBe(false);
  });

  it('loops when asked, so a demo left running does not go silent', () => {
    const frameCount = mockFrames().length;
    const { events, stop } = play(frameCount + 10, { loop: true });
    stop();

    // Play beyond the enlarged fleet script, not just beyond the old single vehicle cycle.
    expect(events.length).toBe(frameCount + 12);
  });
});

describe('mock mission state', () => {
  // The mission panel marks an item active only while MISSION_CURRENT is
  // inside the freshness TTL. The mock has to keep sending it, the same way
  // the backend's rate policy makes a live vehicle keep sending it, or the
  // no-backend display cannot demonstrate the highlight at all.
  it('streams MISSION_CURRENT repeatedly rather than once at startup', () => {
    const cases = mockFrames()
      .map((frame) => frame.build(START_MS))
      .filter((event): event is Extract<StreamEvent, { kind: 'telemetry' }> => event.kind === 'telemetry')
      .filter((event) => event.event.payload.case === 'missionCurrent');

    expect(cases.length).toBeGreaterThan(1);
  });
});

describe('mock family cadence', () => {
  /**
   * Every family the mock sends must arrive inside the freshness TTL.
   *
   * The display withholds any value whose family has not been heard within
   * TELEMETRY_TTL_MS. A fixture that sends a family more slowly than that does
   * not demonstrate a slow sensor — it demonstrates a broken one, permanently,
   * while every other test still passes. This was not hypothetical: the health
   * families sat exactly on the TTL boundary until adding one frame per cycle
   * pushed battery, GPS, EKF and radio into permanent dashes at ?source=mock.
   */
  it('sends every family it sends at all within the freshness TTL', () => {
    const frames = mockFrames();
    const lastSeen = new Map<string, number>();
    const worstGap = new Map<string, number>();
    let elapsedMs = 0;

    for (const frame of [...frames, ...frames]) {
      elapsedMs += frame.delayMs;
      const event = frame.build(START_MS + elapsedMs);
      if (event.kind !== 'telemetry') continue;

      const family = `${String(event.event.vehicleId?.systemId)}:${event.event.payload.case ?? 'unknown'}`;
      const previous = lastSeen.get(family);
      if (previous !== undefined) {
        worstGap.set(family, Math.max(worstGap.get(family) ?? 0, elapsedMs - previous));
      }
      lastSeen.set(family, elapsedMs);
    }

    expect(worstGap.size).toBeGreaterThan(0);

    for (const [family, gap] of worstGap) {
      expect(gap, `${family} is sent every ${String(gap)} ms, outside the ${String(TELEMETRY_TTL_MS)} ms TTL`)
        .toBeLessThan(TELEMETRY_TTL_MS);
    }
  });

  it('sends the three families T-015 added', () => {
    const families = new Set(
      mockFrames()
        .map((frame) => frame.build(START_MS))
        .filter((event): event is Extract<StreamEvent, { kind: 'telemetry' }> => event.kind === 'telemetry')
        .map((event) => event.event.payload.case),
    );

    expect(families).toContain('navControllerOutput');
    expect(families).toContain('homePosition');
    expect(families).toContain('radioStatus');
  });
});

it('folds two distinct fresh vehicles and a lost observer through the real reducer', async () => {
  const { fleetReducer, initialFleetState } = await import('@/fleet/state');
  const { FleetEventType } = await import('@/gen/gcs/v1/fleet_pb');
  let state = initialFleetState;
  let now = START_MS;
  for (const frame of mockFrames(6)) {
    state = fleetReducer(state, { type: 'stream', event: frame.build(now) });
    now += frame.delayMs;
  }
  expect(state.order).toEqual(['1:1', '2:1', '3:1']);
  expect(state.vehicles['3:1']?.lifecycle).toBe(FleetEventType.VEHICLE_LOST);
  expect(state.vehicles['3:1']?.track).toEqual([]);
  const first = state.vehicles['1:1'];
  const second = state.vehicles['2:1'];
  expect(first?.track.length).toBeGreaterThan(0);
  expect(second?.track.length).toBe(first?.track.length);
  expect(second?.track[second.track.length - 1]?.latDeg).not.toBe(first?.track[first.track.length - 1]?.latDeg);
});
