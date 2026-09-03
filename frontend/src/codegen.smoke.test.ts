/** Runtime round-trip smoke test for generated protobuf types. */
import { create, fromBinary, toBinary, toJson } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { AttitudeSchema, TelemetryEventSchema } from '@/gen/gcs/v1/telemetry_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

describe('generated protobuf types', () => {
  it('round-trips a TelemetryEvent through binary', () => {
    const event = create(TelemetryEventSchema, {
      vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
      payload: {
        case: 'attitude',
        value: create(AttitudeSchema, { rollRad: 0.5, pitchRad: -0.25, yawRad: 1.5 }),
      },
    });

    const decoded = fromBinary(TelemetryEventSchema, toBinary(TelemetryEventSchema, event));

    expect(decoded.vehicleId?.systemId).toBe(1);
    expect(decoded.payload.case).toBe('attitude');
    if (decoded.payload.case !== 'attitude') throw new Error('unreachable');
    expect(decoded.payload.value.rollRad).toBeCloseTo(0.5);
  });

  it('carries vehicle identity on the envelope, not the payload', () => {
    const attitude = create(AttitudeSchema, {});
    expect(Object.keys(attitude)).not.toContain('vehicleId');
  });

  // The browser reads mission snapshots as protobuf JSON over HTTP, so the
  // empty-mission posture has to survive that encoding on this side too.
  // A completed mission with no waypoints must not be readable as a failed or
  // still-loading download.
  it('reads a completed empty mission as complete, not absent', () => {
    const snapshot = create(MissionSnapshotSchema, {
      vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
      observedAt: { seconds: 1772366400n, nanos: 0 },
    });

    const json = toJson(MissionSnapshotSchema, snapshot) as Record<string, unknown>;

    // proto3 omits an empty repeated field, so "no waypoints" is carried by the
    // absence of items alongside a present observedAt.
    expect(json).not.toHaveProperty('items');
    expect(json).toHaveProperty('observedAt');
    expect(snapshot.items).toEqual([]);
  });

  it('keeps mission items in the order the vehicle reported them', () => {
    const snapshot = create(MissionSnapshotSchema, {
      items: [{ seq: 0 }, { seq: 1 }, { seq: 2 }],
    });

    const decoded = fromBinary(
      MissionSnapshotSchema,
      toBinary(MissionSnapshotSchema, snapshot),
    );

    expect(decoded.items.map((item) => item.seq)).toEqual([0, 1, 2]);
  });

});
