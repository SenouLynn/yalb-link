/** Reads the backend's observation time off a telemetry payload. */

import type { Timestamp } from '@bufbuild/protobuf/wkt';

import { TelemetryEventSchema, type TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';

const NANOS_PER_MS = 1e6;
const MS_PER_SECOND = 1000;

/**
 * Milliseconds since the epoch, or `null` when the payload carries no stamp.
 *
 * Resolved through the schema's oneof rather than a switch over the payload
 * cases, so a telemetry family added later is covered without this file
 * changing — the same reason the backend stamps by descriptor.
 */
export function observedAtMs(event: TelemetryEvent): number | null {
  const field = event.payload.value;

  if (field === undefined || typeof field !== 'object') {
    return null;
  }

  const stamp = (field as { observedAt?: Timestamp | undefined }).observedAt;

  if (stamp === undefined) {
    return null;
  }

  return Number(stamp.seconds) * MS_PER_SECOND + stamp.nanos / NANOS_PER_MS;
}

/** Every telemetry payload declares observed_at; asserted by the contract test. */
export const TELEMETRY_PAYLOAD_ONEOF = TelemetryEventSchema.oneofs.find(
  (oneof) => oneof.name === 'payload',
);
