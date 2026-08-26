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
  private stopped = false;
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
    this.onEvent = onEvent;
    this.stopped = false;
    this.lastWallMs = this.wallNow();

    // Replay is always "connected": there is no transport to lose, and a
    // connection warning here would correspond to nothing. Whether the data is
    // live is a separate question, answered by the source chip.
    onEvent({ kind: 'connection', connected: true, receivedAtMs: this.wallNow() });

    this.loadPromise = this.loadAll();
    this.scheduleTick();

    return () => {
      this.stopped = true;
      this.onEvent = null;
      this.cancelTick?.();
      this.cancelTick = null;
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
    this.apply(play(this.playback));
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

  private scheduleTick(): void {
    if (this.stopped) {
      return;
    }

    this.cancelTick = this.schedule(() => {
      this.cancelTick = null;

      if (this.stopped) {
        return;
      }

      this.tick();
      this.scheduleTick();
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

  private async loadAll(): Promise<void> {
    this.loading = true;
    this.error = null;
    this.notify();

    try {
      let fromSeq = 0;

      for (;;) {
        const page = await this.fetchPage(this.recordingId, fromSeq, this.pageSize);
        this.recording = page.recording;
        this.absorb(page.events);

        if (page.next_seq === null || this.buffer.length >= MAX_BUFFERED_EVENTS) {
          this.truncated = page.next_seq !== null;
          break;
        }

        fromSeq = page.next_seq;
      }
    } catch (cause) {
      this.error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      this.loading = false;
      // Show the recording's opening state rather than a blank display: an
      // empty screen waiting on play looks like a recording that failed.
      this.drain();
      this.notify();
    }
  }

  /** Parses one page into the buffer and grows the timeline to fit it. */
  private absorb(events: ReplayPageEventWire[]): void {
    for (const wire of events) {
      const name = wire.kind === 'fleet' ? EVENT_FLEET : EVENT_TELEMETRY;
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
  }
}
