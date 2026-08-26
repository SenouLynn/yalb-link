import { describe, expect, it, vi } from 'vitest';

import { toJson } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { create } from '@bufbuild/protobuf';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { AttitudeSchema, TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

import type { StreamEvent } from './events';
import { ReplayEventSource, type ReplayPageWire } from './replay';
import type { Cancel, Scheduler } from './scheduler';

const START = 1_700_000_000_000;
const VEHICLE = create(VehicleIdSchema, { systemId: 7, componentId: 1 });

/** A scheduler that fires only when the test says so. */
function manualScheduler(): { schedule: Scheduler; tick: () => void; pending: () => number } {
  let queue: (() => void)[] = [];

  const schedule: Scheduler = (fn) => {
    queue.push(fn);

    return () => {
      queue = queue.filter((entry) => entry !== fn);
    };
  };

  return {
    schedule,
    pending: () => queue.length,
    tick: () => {
      const due = queue;
      queue = [];
      for (const fn of due) fn();
    },
  };
}

function attitudeEventJson(atMs: number, rollRad: number): unknown {
  const event = create(TelemetryEventSchema, {
    vehicleId: VEHICLE,
    payload: {
      case: 'attitude',
      value: create(AttitudeSchema, { rollRad, observedAt: timestampFromMs(atMs) }),
    },
  });

  return toJson(TelemetryEventSchema, event);
}

function fleetEventJson(atMs: number): unknown {
  const event = create(FleetEventSchema, {
    type: FleetEventType.VEHICLE_DISCOVERED,
    vehicleId: VEHICLE,
    occurredAt: timestampFromMs(atMs),
  });

  return toJson(FleetEventSchema, event);
}

/** Builds a two-page recording: one fleet event then three attitudes. */
function pages(): ReplayPageWire[] {
  const recording = {
    id: 1,
    name: 'test flight',
    status: 'stopped',
    started_at: new Date(START).toISOString(),
    stopped_at: new Date(START + 3_000).toISOString(),
    stop_reason: 'requested',
    event_count: 4,
  };

  return [
    {
      recording,
      next_seq: 3,
      events: [
        { seq: 1, kind: 'fleet', occurred_at_ms: START, event: fleetEventJson(START) },
        { seq: 2, kind: 'telemetry', occurred_at_ms: START + 1_000, event: attitudeEventJson(START + 1_000, 0.1) },
      ],
    },
    {
      recording,
      next_seq: null,
      events: [
        { seq: 3, kind: 'telemetry', occurred_at_ms: START + 2_000, event: attitudeEventJson(START + 2_000, 0.2) },
        { seq: 4, kind: 'telemetry', occurred_at_ms: START + 3_000, event: attitudeEventJson(START + 3_000, 0.3) },
      ],
    },
  ];
}

function newSource(overrides: { fetchPage?: ReturnType<typeof vi.fn> } = {}) {
  const scheduler = manualScheduler();
  const script = pages();
  const fetchPage =
    overrides.fetchPage ??
    vi.fn((_recordingId: number, fromSeq: number) =>
      Promise.resolve(fromSeq === 0 ? script[0] : script[1]),
    );

  let wallMs = 0;
  const source = new ReplayEventSource({
    recordingId: 1,
    fetchPage: fetchPage as never,
    schedule: scheduler.schedule,
    now: () => wallMs,
  });

  return {
    source,
    scheduler,
    fetchPage,
    advanceWall: (ms: number) => {
      wallMs += ms;
    },
  };
}

function collect(source: ReplayEventSource): { events: StreamEvent[]; stop: Cancel } {
  const events: StreamEvent[] = [];
  const stop = source.start((event) => events.push(event));

  return { events, stop };
}

describe('ReplayEventSource', () => {
  it('reports itself connected: there is no transport to lose', async () => {
    const { source } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    expect(events[0]).toMatchObject({ kind: 'connection', connected: true });
    stop();
  });

  it('pages until next_seq is null', async () => {
    const { source, fetchPage } = newSource();
    const { stop } = collect(source);
    await source.loaded();

    expect(fetchPage).toHaveBeenCalledTimes(2);
    expect(fetchPage.mock.calls[0]?.[1]).toBe(0);
    expect(fetchPage.mock.calls[1]?.[1]).toBe(3);
    expect(source.status().totalEvents).toBe(4);
    stop();
  });

  it('exposes the recording timeline as its clock, not the wall clock', async () => {
    const { source } = newSource();
    const { stop } = collect(source);
    await source.loaded();

    // The whole point: freshness is judged against observed_at, so the clock
    // the display reads has to be on the recording's timeline.
    expect(source.now()).toBe(START);
    expect(source.status().endedAtMs).toBe(START + 3_000);
    stop();
  });

  it('shows the first recorded frame as soon as it loads', async () => {
    const { source } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    // Sitting on a blank display until the operator presses play would look
    // like a recording that failed to load.
    expect(events.filter((event) => event.kind !== 'connection')).toHaveLength(1);
    stop();
  });

  it('emits events as the clock passes them, in recorded order', async () => {
    const { source, scheduler, advanceWall } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    source.play();
    advanceWall(1_500);
    scheduler.tick();

    const emitted = events.filter((event) => event.kind !== 'connection');
    expect(emitted).toHaveLength(2);
    expect(emitted[0]?.kind).toBe('fleet');
    expect(emitted[1]?.kind).toBe('telemetry');
    expect(source.now()).toBe(START + 1_500);

    advanceWall(1_000);
    scheduler.tick();
    expect(events.filter((event) => event.kind !== 'connection')).toHaveLength(3);
    stop();
  });

  it('emits nothing further while paused', async () => {
    const { source, scheduler, advanceWall } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();
    const loaded = events.length;

    advanceWall(5_000);
    scheduler.tick();

    expect(events).toHaveLength(loaded);
    expect(source.now()).toBe(START);
    stop();
  });

  it('seeking forward catches up without a reset', async () => {
    const { source, scheduler } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    source.seekTo(START + 2_000);
    scheduler.tick();

    expect(events.some((event) => event.kind === 'reset')).toBe(false);
    expect(events.filter((event) => event.kind === 'telemetry' || event.kind === 'fleet')).toHaveLength(3);
    stop();
  });

  it('seeking backward resets and replays from the beginning', async () => {
    const { source, scheduler } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    source.seekTo(START + 3_000);
    scheduler.tick();
    const beforeSeek = events.length;

    source.seekTo(START + 1_000);
    scheduler.tick();

    // The map track only ever appends, so rewinding has to rebuild it from
    // the start rather than leave a tail from a future that no longer applies.
    const afterSeek = events.slice(beforeSeek);
    expect(afterSeek[0]?.kind).toBe('reset');
    expect(afterSeek.filter((event) => event.kind === 'fleet' || event.kind === 'telemetry')).toHaveLength(2);
    stop();
  });

  it('returns a stable status snapshot until something actually changes', async () => {
    const { source, advanceWall, scheduler } = newSource();
    const { stop } = collect(source);
    await source.loaded();

    // useSyncExternalStore requires getSnapshot to be cached: a fresh object
    // on every call is an infinite re-render loop, and the display renders
    // nothing at all.
    expect(source.status()).toBe(source.status());

    const before = source.status();
    source.play();
    expect(source.status()).not.toBe(before);
    expect(source.status()).toBe(source.status());

    const playing = source.status();
    advanceWall(500);
    scheduler.tick();
    expect(source.status()).not.toBe(playing);
    expect(source.status().positionMs).toBe(START + 500);
    stop();
  });

  it('stops scheduling once cancelled', async () => {
    const { source, scheduler } = newSource();
    const { stop } = collect(source);
    await source.loaded();

    expect(scheduler.pending()).toBe(1);
    stop();
    expect(scheduler.pending()).toBe(0);
  });

  it('surfaces a failed load as an error instead of throwing', async () => {
    const fetchPage = vi.fn(() => Promise.reject(new Error('backend is down')));
    const { source } = newSource({ fetchPage });
    const { stop } = collect(source);
    await source.loaded();

    expect(source.status().error).toContain('backend is down');
    expect(source.status().totalEvents).toBe(0);
    stop();
  });

  it('drops an unparseable event rather than failing the recording', async () => {
    const script = pages();
    const broken = { ...script[0], events: [{ seq: 1, kind: 'telemetry', occurred_at_ms: START, event: { nope: true } }] };
    const fetchPage = vi.fn((_id: number, fromSeq: number) =>
      Promise.resolve(fromSeq === 0 ? broken : script[1]),
    );
    const { source, scheduler } = newSource({ fetchPage: fetchPage as never });
    const { events, stop } = collect(source);
    await source.loaded();

    source.seekTo(START + 3_000);
    scheduler.tick();

    expect(source.status().totalEvents).toBe(2);
    expect(events.filter((event) => event.kind === 'telemetry')).toHaveLength(2);
    stop();
  });
});
