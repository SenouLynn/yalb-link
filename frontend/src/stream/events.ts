/**
 * The browser's view of the backend event stream.
 *
 * Live SSE and deterministic mock playback are the same shape on purpose: the
 * display is the thing under test in both cases, and a mock that took a
 * different path through the app would prove nothing about the live one.
 */

import type { FleetEvent } from '@/gen/gcs/v1/fleet_pb';
import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';

/** One thing that arrived from the backend, stamped on receipt. */
export type StreamEvent =
  | { kind: 'fleet'; event: FleetEvent; receivedAtMs: number }
  | { kind: 'telemetry'; event: TelemetryEvent; receivedAtMs: number }
  /**
   * Transport state, not vehicle state. A vehicle can be perfectly healthy
   * while the browser has lost its connection to the backend, and conflating
   * the two would show the operator a lost aircraft when the real fault is a
   * restarted server.
   */
  | { kind: 'connection'; connected: boolean; receivedAtMs: number };

/** A source of stream events. Implementations are interchangeable. */
export interface TelemetryStream {
  /** Starts delivery and returns the function that stops it. */
  start(onEvent: (event: StreamEvent) => void): () => void;
}

/** SSE event names, matching `internal/stream`. */
export const EVENT_FLEET = 'fleet';
export const EVENT_TELEMETRY = 'telemetry';
