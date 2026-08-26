/**
 * The browser's view of the backend event stream.
 *
 * Live SSE and deterministic mock playback are the same shape on purpose: the
 * display is the thing under test in both cases, and a mock that took a
 * different path through the app would prove nothing about the live one.
 */

import type { FleetEvent } from '@/gen/gcs/v1/fleet_pb';
import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';
import type { CommandTransaction } from '@/gen/gcs/v1/commands_pb';

/** One thing that arrived from the backend, stamped on receipt. */
export type StreamEvent =
  | { kind: 'fleet'; event: FleetEvent; receivedAtMs: number }
  | { kind: 'telemetry'; event: TelemetryEvent; receivedAtMs: number }
  | { kind: 'command'; event: CommandTransaction; receivedAtMs: number }
  /**
   * Transport state, not vehicle state. A vehicle can be perfectly healthy
   * while the browser has lost its connection to the backend, and conflating
   * the two would show the operator a lost aircraft when the real fault is a
   * restarted server.
   */
  | { kind: 'connection'; connected: boolean; receivedAtMs: number }
  /**
   * Discard everything accumulated so far; what follows starts over.
   *
   * Replay needs this because the map track only ever appends: rewinding
   * cannot subtract the points from a future the operator has just seeked away
   * from, so the trail is rebuilt from the beginning instead.
   */
  | { kind: 'reset'; receivedAtMs: number };

/** A source of stream events. Implementations are interchangeable. */
export interface TelemetryStream {
  /** Starts delivery and returns the function that stops it. */
  start(onEvent: (event: StreamEvent) => void): () => void;

  /**
   * The clock freshness should be judged against, when the source owns one.
   *
   * Live and mock leave this unset and the display uses the wall clock, which
   * is right for data arriving now. Replay implements it and returns its
   * position on the recording's timeline, because recorded telemetry carries
   * the timestamps of the flight that produced it — measured against the wall
   * clock, every reading in a replay would be hours stale.
   */
  now?(): number;
}

/** SSE event names, matching `internal/stream`. */
export const EVENT_FLEET = 'fleet';
export const EVENT_TELEMETRY = 'telemetry';
export const EVENT_COMMAND = 'command';
