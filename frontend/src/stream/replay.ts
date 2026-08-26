/**
 * Playback of a persisted recording, through the live stream's own interface.
 *
 * Pacing is the browser's rather than the backend's. The server serves bounded
 * pages of recorded events and this class owns the clock, so scrubbing,
 * pausing, and changing speed are local state rather than a reconnect — and
 * the ordering rules that decide what the display sees stay in
 * `logic/playback`, testable without a DOM or a network.
 */

import {
  advance,
  createPlayback,
  dueCount,
  extendTo,
  pause,
  play,
  seekTo,
  setSpeed,
  type PlaybackState,
} from '@/logic/playback';

import { EVENT_FLEET, EVENT_TELEMETRY, type StreamEvent, type TelemetryStream } from './events';
import { parseStreamJson } from './parse';
import { defaultScheduler, type Cancel, type Scheduler } from './scheduler';

/** Where recordings are served. Relative, for the reason `live.ts` gives. */
export const RECORDINGS_PATH = '/api/recordings';

/** Events requested per page. The backend clamps anything larger. */
export const DEFAULT_PAGE_SIZE = 2000;

/**
 * How many events one replay will hold in the browser.
 *
 * A recording is capped at 200,000 events server-side, which is a sane bound
 * for a disk file and a poor one for a tab. Stopping early and saying so beats
 * either exhausting memory or silently showing a partial flight as a whole one.
 */
export const MAX_BUFFERED_EVENTS = 50_000;

/** How often playback advances. Fine enough that 1 Hz families look smooth. */
export const REPLAY_TICK_MS = 100;

/** One recording's lifecycle metadata, as the backend serves it. */
export interface RecordingWire {
  id: number;
  name: string;
  status: string;
  started_at: string;
  stopped_at?: string;
  stop_reason?: string;
  event_count: number;
}

/** One recorded event in a page. `event` is protobuf JSON. */
export interface ReplayPageEventWire {
  seq: number;
  kind: string;
  occurred_at_ms: number;
  event: unknown;
}

/** One page of `GET /api/recordings/{id}/events`. */
export interface ReplayPageWire {
  recording: RecordingWire;
  events: ReplayPageEventWire[];
  next_seq: number | null;
}

/** An HTTP failure while mutating a recording. */
export class RecordingHTTPError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'RecordingHTTPError';
  }
}

/** Permanently deletes one stopped recording. */
export async function deleteRecording(id: number): Promise<void> {
  const response = await globalThis.fetch(`${RECORDINGS_PATH}/${String(id)}`, { method: 'DELETE' });
  if (!response.ok) {
    throw new RecordingHTTPError(
      `Deleting recording ${String(id)} returned HTTP ${String(response.status)}.`,
      response.status,
    );
  }
}

/** Reads one page. Injected so tests need no network. */
export type FetchPage = (recordingId: number, fromSeq: number, limit: number) => Promise<ReplayPageWire>;

/** What the replay controls render from. */
export interface ReplayStatus extends PlaybackState {
  recording: RecordingWire | null;
  /** Events buffered and playable. Excludes any that failed to parse. */
  totalEvents: number;
  loading: boolean;
  /** True when the recording was longer than this browser will hold. */
  truncated: boolean;
  error: string | null;
}

export interface ReplayOptions {
  recordingId: number;
  /** Injected page reader. Defaults to the backend endpoint. */
  fetchPage?: FetchPage;
  /** Injected scheduler. Defaults to setTimeout. */
  schedule?: Scheduler;
  /** Injected *wall* clock, used only to measure elapsed real time. */
  now?: () => number;
  tickMs?: number;
  pageSize?: number;
}

interface BufferedEvent {
  atMs: number;
  event: StreamEvent;
}

/** The default page reader, over the browser's own fetch. */
async function fetchPageOverHTTP(
  recordingId: number,
  fromSeq: number,
  limit: number,
): Promise<ReplayPageWire> {
  const query = new URLSearchParams({ from_seq: String(fromSeq), limit: String(limit) });
  const response = await globalThis.fetch(`${RECORDINGS_PATH}/${String(recordingId)}/events?${query.toString()}`);

  if (!response.ok) {
    throw new Error(`replay: recording ${String(recordingId)} returned HTTP ${String(response.status)}`);
  }

  return (await response.json()) as ReplayPageWire;
}

export class ReplayEventSource implements TelemetryStream {
  private readonly recordingId: number;
  private readonly fetchPage: FetchPage;
  private readonly schedule: Scheduler;
  private readonly wallNow: () => number;
  private readonly tickMs: number;
  private readonly pageSize: number;

  private playback: PlaybackState = createPlayback(0, 0);
  private buffer: BufferedEvent[] = [];
  private cursor = 0;
  private recording: RecordingWire | null = null;
  private loading = false;
  private truncated = false;
  private error: string | null = null;

  private onEvent: ((event: StreamEvent) => void) | null = null;
  private cancelTick: Cancel | null = null;
  private lastWallMs = 0;
  /** Identifies the latest start, so an older async load cannot mutate it. */
  private generation = 0;
  private activeGeneration = 0;
  private loadPromise: Promise<void> = Promise.resolve();
  private snapshot: ReplayStatus | null = null;
  private readonly listeners = new Set<() => void>();

  constructor(options: ReplayOptions) {
    this.recordingId = options.recordingId;
    this.fetchPage = options.fetchPage ?? fetchPageOverHTTP;
    this.schedule = options.schedule ?? defaultScheduler;
    this.wallNow = options.now ?? (() => Date.now());
    this.tickMs = options.tickMs ?? REPLAY_TICK_MS;
    this.pageSize = options.pageSize ?? DEFAULT_PAGE_SIZE;
  }

  start(onEvent: (event: StreamEvent) => void): Cancel {
    const generation = ++this.generation;
    this.activeGeneration = generation;
    this.onEvent = onEvent;
    this.cancelTick?.();
    this.cancelTick = null;
    this.playback = createPlayback(0, 0);
    this.buffer = [];
    this.cursor = 0;
    this.recording = null;
    this.loading = false;
    this.truncated = false;
    this.error = null;
    this.lastWallMs = this.wallNow();
    this.notify();

    // Replay is always "connected": there is no transport to lose, and a
    // connection warning here would correspond to nothing. Whether the data is
    // live is a separate question, answered by the source chip.
    onEvent({ kind: 'connection', connected: true, receivedAtMs: this.wallNow() });

    this.loadPromise = this.loadAll(generation);
    this.scheduleTick(generation);

    return () => {
      if (this.activeGeneration === generation) {
        this.activeGeneration = 0;
        this.onEvent = null;
        this.cancelTick?.();
        this.cancelTick = null;
      }
    };
  }

  /** The recording's own clock. This is what the display measures against. */
  now(): number {
    return this.playback.positionMs;
  }

  /** Resolves once paging has finished, successfully or not. */
  loaded(): Promise<void> {
    return this.loadPromise;
  }

  /**
   * The current status, as one cached object.
   *
   * Cached because React's `useSyncExternalStore` compares snapshots by
   * identity: a freshly built object on every call is an infinite re-render
   * loop, and the display renders nothing at all. The cache is dropped
   * whenever something notifies, so a stale snapshot cannot outlive a change.
   */
  status = (): ReplayStatus => {
    this.snapshot ??= {
      ...this.playback,
      recording: this.recording,
      totalEvents: this.buffer.length,
      loading: this.loading,
      truncated: this.truncated,
      error: this.error,
    };

    return this.snapshot;
  };

  /** Notifies on any state change, so the controls can re-render. */
  subscribe = (listener: () => void): Cancel => {
    this.listeners.add(listener);

    return () => {
      this.listeners.delete(listener);
    };
  };

  play(): void {
    // Elapsed real time while paused is not playback time; without this, a
    // recording resumed after a minute would jump a minute forward.
    this.lastWallMs = this.wallNow();
    const wasFinished = this.playback.positionMs >= this.playback.endedAtMs;
    const next = play(this.playback);

    if (wasFinished && next.positionMs < this.playback.positionMs) {
      this.playback = next;
      this.cursor = 0;
      this.emit({ kind: 'reset', receivedAtMs: this.wallNow() });
      this.drain();
      this.notify();
      return;
    }

    this.apply(next);
  }

  pause(): void {
    this.apply(pause(this.playback));
  }

  setSpeed(speed: number): void {
    this.apply(setSpeed(this.playback, speed));
  }

  seekTo(positionMs: number): void {
    const target = seekTo(this.playback, positionMs);

    if (target === this.playback || target.positionMs === this.playback.positionMs) {
      return;
    }

    const backward = target.positionMs < this.playback.positionMs;
    this.playback = target;

    if (backward) {
      this.cursor = 0;
      this.emit({ kind: 'reset', receivedAtMs: this.wallNow() });
    }

    this.drain();
    this.notify();
  }

  private apply(next: PlaybackState): void {
    if (next === this.playback) {
      return;
    }

    this.playback = next;
    this.notify();
  }

  private scheduleTick(generation: number): void {
    if (this.activeGeneration !== generation) {
      return;
    }

    this.cancelTick = this.schedule(() => {
      this.cancelTick = null;

      if (this.activeGeneration !== generation) {
        return;
      }

      this.tick();
      this.scheduleTick(generation);
    }, this.tickMs);
  }

  private tick(): void {
    const wall = this.wallNow();
    const delta = wall - this.lastWallMs;
    this.lastWallMs = wall;

    this.apply(advance(this.playback, delta));
    this.drain();
  }

  /** Emits every buffered event the clock has now passed. */
  private drain(): void {
    const due = dueCount(this.buffer, this.cursor, this.playback.positionMs);

    for (let index = 0; index < due; index += 1) {
      const item = this.buffer[this.cursor + index];

      if (item !== undefined) {
        this.emit(item.event);
      }
    }

    this.cursor += due;
  }

  private emit(event: StreamEvent): void {
    this.onEvent?.(event);
  }

  private notify(): void {
    this.snapshot = null;

    for (const listener of this.listeners) {
      listener();
    }
  }

  private async loadAll(generation: number): Promise<void> {
    this.loading = true;
    this.error = null;
    this.notify();

    try {
      let fromSeq = 0;

      for (;;) {
        const page = await this.fetchPage(this.recordingId, fromSeq, this.pageSize);

        // React Strict Mode deliberately restarts effects in development. A
        // response belonging to the cleaned-up start must not append into the
        // replacement start's buffer.
        if (this.activeGeneration !== generation) {
          return;
        }

        this.recording = page.recording;
        const overflowed = this.absorb(page.events);

        if (overflowed || page.next_seq === null || this.buffer.length >= MAX_BUFFERED_EVENTS) {
          this.truncated = overflowed || page.next_seq !== null;
          break;
        }

        if (!Number.isSafeInteger(page.next_seq) || page.next_seq <= fromSeq) {
          throw new Error('replay: backend returned a non-advancing next_seq');
        }
        fromSeq = page.next_seq;
      }
    } catch (cause) {
      if (this.activeGeneration === generation) {
        this.error = cause instanceof Error ? cause.message : String(cause);
      }
    } finally {
      if (this.activeGeneration === generation) {
        this.loading = false;
        // Show the recording's opening state rather than a blank display: an
        // empty screen waiting on play looks like a recording that failed.
        this.drain();
        this.notify();
      }
    }
  }

  /** Parses one page into the buffer. Returns true if the cap cut it short. */
  private absorb(events: ReplayPageEventWire[]): boolean {
    for (const wire of events) {
      if (this.buffer.length >= MAX_BUFFERED_EVENTS) {
        return true;
      }

      const name = wire.kind === 'fleet'
        ? EVENT_FLEET
        : wire.kind === 'telemetry'
          ? EVENT_TELEMETRY
          : null;
      if (name === null) {
        continue;
      }
      const parsed = parseStreamJson(name, wire.event as never, wire.occurred_at_ms);

      if (parsed === null) {
        continue;
      }

      if (this.buffer.length === 0) {
        this.playback = createPlayback(wire.occurred_at_ms, wire.occurred_at_ms);
      }

      this.buffer.push({ atMs: wire.occurred_at_ms, event: parsed });
      this.playback = extendTo(this.playback, wire.occurred_at_ms);
    }

    return false;
  }
}
