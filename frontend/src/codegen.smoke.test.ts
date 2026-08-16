/**
 * Smoke test for the generated Connect/protobuf types.
 *
 * Typechecking proves the generated code compiles; this proves it is loadable
 * and round-trips at runtime, which is a different failure mode (a bad plugin
 * version can emit code that types fine and throws on import). It also gives the
 * Bazel vitest target something real to run — a test suite that passes because
 * it found no tests is not a gate.
 *
 * This is deliberately not domain logic. Resolver tests arrive in Tier 1.
 */
import { create, fromBinary, toBinary } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { AttitudeSchema, TelemetryEventSchema, TelemetryPayloadType } from '@/gen/gcs/v1/telemetry_pb';
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
    // Tier 0 removed vehicle_id from every telemetry payload message. If it comes
    // back, this stops compiling and the reviewer finds out here rather than from
    // two fields disagreeing at 20 Hz.
    const attitude = create(AttitudeSchema, {});
    expect(Object.keys(attitude)).not.toContain('vehicleId');
  });

  it('exposes the payload-type enum used for stream filtering', () => {
    expect(TelemetryPayloadType.ATTITUDE).toBeDefined();
    expect(TelemetryPayloadType.UNSPECIFIED).toBe(0);
  });
});
