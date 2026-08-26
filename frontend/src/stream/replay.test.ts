import { describe, expect, it, vi } from 'vitest';

import { toJson } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { create } from '@bufbuild/protobuf';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { AttitudeSchema, TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

import type { StreamEvent } from './events';
import {
  deleteRecording,
  MAX_BUFFERED_EVENTS,
  ReplayEventSource,
  type ReplayPageWire,
} from './replay';
import type { Cancel, Scheduler } from './scheduler';

const START = 1_700_000_000_000;
const VEHICLE = create(VehicleIdSchema, { systemId: 7, componentId: 1 });

describe('deleteRecording', () => {
  it('sends DELETE and accepts 204', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));

    await expect(deleteRecording(7)).resolves.toBeUndefined();
    expect(fetch).toHaveBeenCalledWith('/api/recordings/7', { method: 'DELETE' });
    fetch.mockRestore();
  });

  it.each([404, 409])('preserves HTTP status %d', async (status) => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status }));

    await expect(deleteRecording(7)).rejects.toMatchObject({ status });
    fetch.mockRestore();
  });
});

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

  it('rebuilds from the beginning when Play restarts a finished replay', async () => {
    const { source, scheduler, advanceWall } = newSource();
    const { events, stop } = collect(source);
    await source.loaded();

    source.play();
    advanceWall(3_000);
    scheduler.tick();
    expect(source.status().playing).toBe(false);
    const beforeRestart = events.length;

    source.play();

    const restarted = events.slice(beforeRestart);
    expect(restarted[0]?.kind).toBe('reset');
    expect(restarted.filter((event) => event.kind === 'fleet' || event.kind === 'telemetry')).toHaveLength(1);
    expect(source.now()).toBe(START);
    expect(source.status().playing).toBe(true);
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

  it('ignores a page load from a cleaned-up Strict Mode start', async () => {
    const script = pages();
    const firstPage = script[0];
    if (firstPage === undefined) throw new Error('test fixture has no first page');
    const resolvers: ((page: ReplayPageWire) => void)[] = [];
    const fetchPage = vi.fn(() => new Promise<ReplayPageWire>((resolve) => resolvers.push(resolve)));
    const { source } = newSource({ fetchPage });

    const first = collect(source);
    first.stop();
    const second = collect(source);

    resolvers[0]?.(firstPage);
    await Promise.resolve();
    expect(fetchPage).toHaveBeenCalledTimes(2);

    resolvers[1]?.({ ...firstPage, next_seq: null });
    await source.loaded();

    expect(source.status().totalEvents).toBe(2);
    second.stop();
  });

  it('fails a non-advancing page cursor instead of requesting forever', async () => {
    const firstPage = pages()[0];
    if (firstPage === undefined) throw new Error('test fixture has no first page');
    const first = { ...firstPage, next_seq: 0 };
    const fetchPage = vi.fn(() => Promise.resolve(first));
    const { source } = newSource({ fetchPage });
    const { stop } = collect(source);
    await source.loaded();

    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(source.status().error).toContain('non-advancing next_seq');
    stop();
  });

  it('never buffers more than the documented browser cap', async () => {
    const firstPage = pages()[0];
    const event = firstPage?.events[0];
    if (firstPage === undefined || event === undefined) throw new Error('test fixture is empty');
    const oversized = {
      ...firstPage,
      next_seq: null,
      events: Array.from({ length: MAX_BUFFERED_EVENTS + 1 }, (_, index) => ({
        ...event,
        seq: index + 1,
      })),
    };
    const fetchPage = vi.fn(() => Promise.resolve(oversized));
    const { source } = newSource({ fetchPage });
    const { stop } = collect(source);
    await source.loaded();

    expect(source.status().totalEvents).toBe(MAX_BUFFERED_EVENTS);
    expect(source.status().truncated).toBe(true);
    stop();
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
