/** Turns backend frames back into the protobuf messages the backend sent. */

import { fromJson, fromJsonString, type JsonValue } from '@bufbuild/protobuf';

import { FleetEventSchema } from '@/gen/gcs/v1/fleet_pb';
import { TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';

import { EVENT_FLEET, EVENT_TELEMETRY, type StreamEvent } from './events';

/**
 * Parses one SSE frame.
 *
 * Returns `null` rather than throwing for anything unrecognised. A single
 * malformed or unknown frame must not take the display down: the stream is
 * long-lived, the backend may learn to send new event names before this client
 * learns to read them, and dropping one frame costs at most one update of a
 * value that is about to be sent again.
 */
export function parseStreamEvent(
  name: string,
  data: string,
  receivedAtMs: number,
): StreamEvent | null {
  try {
    switch (name) {
      case EVENT_FLEET:
        return {
          kind: 'fleet',
          event: fromJsonString(FleetEventSchema, data),
          receivedAtMs,
        };

      case EVENT_TELEMETRY:
        return {
          kind: 'telemetry',
          event: fromJsonString(TelemetryEventSchema, data),
          receivedAtMs,
        };

      default:
        return null;
    }
  } catch {
    return null;
  }
}

/**
 * Parses one already-decoded protobuf-JSON payload.
 *
 * The replay endpoint returns its events inside a JSON envelope rather than as
 * SSE text, so they arrive parsed. Everything else is identical to
 * `parseStreamEvent`, including returning `null` for anything unrecognised: a
 * recording with one unreadable event is still worth watching.
 */
export function parseStreamJson(
  name: string,
  value: JsonValue,
  receivedAtMs: number,
): StreamEvent | null {
  try {
    switch (name) {
      case EVENT_FLEET:
        return { kind: 'fleet', event: fromJson(FleetEventSchema, value), receivedAtMs };

      case EVENT_TELEMETRY:
        return { kind: 'telemetry', event: fromJson(TelemetryEventSchema, value), receivedAtMs };

      default:
        return null;
    }
  } catch {
    return null;
  }
}
