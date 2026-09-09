import { describe, expect, it } from 'vitest';

import type { StreamEvent } from './events';
import { MOCK_COMP_ID, MOCK_SYS_ID, mockFrames } from './fixtures';
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

    const kinds = events.filter((e) => e.kind !== 'connection').map((e) => e.kind);

    expect(kinds[0]).toBe('fleet');
    expect(kinds.slice(1).every((kind) => kind === 'telemetry')).toBe(true);
  });

  it('carries the mock vehicle identity on every event', () => {
    const { events, stop } = play(10);
    stop();

    for (const event of events) {
      if (event.kind === 'connection' || event.kind === 'reset') continue;

      expect(event.event.vehicleId?.systemId).toBe(MOCK_SYS_ID);
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
    const { events, stop } = play(400, { loop: true });
    stop();

    // The bounded script is shorter than 400 steps; looping keeps producing.
    expect(events.length).toBeGreaterThan(300);
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

    for (const frame of frames) {
      elapsedMs += frame.delayMs;
      const event = frame.build(START_MS + elapsedMs);
      if (event.kind !== 'telemetry') continue;

      const family = event.event.payload.case ?? 'unknown';
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
