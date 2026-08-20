/** Runtime round-trip smoke test for generated protobuf types. */
import { create, fromBinary, toBinary } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

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

});
