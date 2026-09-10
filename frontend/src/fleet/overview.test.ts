import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { TelemetryEventSchema, GlobalPositionSchema } from '@/gen/gcs/v1/telemetry_pb';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { fleetReducer, initialFleetState } from './state';
import { fleetPositions } from './overview';
const event = (systemId: number, componentId: number, at: number) => ({ type: 'stream' as const, event: {
  kind: 'telemetry' as const, receivedAtMs: at,
  event: create(TelemetryEventSchema, { vehicleId: {systemId, componentId}, payload: {case: 'globalPosition', value: create(GlobalPositionSchema, {latDeg: 37, lonDeg: -122, altRelativeM: 10})} }),
} });
it('gates each full identity independently at the position TTL and recovers without dropping roster entries', () => {
  let state = fleetReducer(initialFleetState, event(1, 2, 0));
  state = fleetReducer(state, event(1, 1, 2000));
  expect(fleetPositions(state, 4999).map(p => p.key)).toEqual(['1:1', '1:2']);
  expect(fleetPositions(state, 5000).map(p => p.key)).toEqual(['1:1']);
  expect(state.order).toHaveLength(2);
  state = fleetReducer(state, event(1, 2, 5001));
  expect(fleetPositions(state, 5001)).toHaveLength(2);
  const view = state.vehicles['1:2'];
  if (!view) throw new Error('missing vehicle');
  state = {...state, vehicles: {...state.vehicles, '1:2': {...view, lifecycle: FleetEventType.VEHICLE_LOST}}};
  expect(fleetPositions(state, 5001).map(p => p.key)).toEqual(['1:1']);
  state = fleetReducer(state, {type:'stream', event: {kind:'reset', receivedAtMs:5002}});
  expect(fleetPositions(state, 5002)).toEqual([]);
  expect(state.order).toEqual([]);
});
