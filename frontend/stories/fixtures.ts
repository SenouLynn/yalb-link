import { fleetReducer, initialFleetState, type VehicleView } from '@/fleet/state';
import { mockFrames } from '@/stream/fixtures';
import { readFlight, UNAVAILABLE, type Reading, type ReadingState } from '@/ui/readings';

export const NOW = 1_700_000_000_000;
export const stateControl = { control: 'select' as const, options: ['live', 'stale', 'unavailable'] };
export function reading<T extends { source: string }>(value: T, state: ReadingState): Reading<T> {
  if (state === 'unavailable') return UNAVAILABLE;
  return { value, state, source: value.source, ageMs: state === 'stale' ? 6000 : 200, ttlMs: 5000 };
}

// Reuse production mock events and folding; freeze time so stories never age out.
export const fixtureFleet = mockFrames(1).reduce((state, frame) =>
  fleetReducer(state, { type: 'stream', event: frame.build(NOW) }), initialFleetState);
export function fixtureVehicle(): VehicleView {
  const view = fixtureFleet.vehicles['1:1'];
  if (!view) throw new Error('Story fixture vehicle missing');
  return view;
}
export const vehicle = fixtureVehicle();
const live = readFlight(vehicle, NOW);
function withState<T>(value: Reading<T>, state: ReadingState): Reading<T> {
  return state === 'unavailable' ? UNAVAILABLE : {
    ...value, state, ageMs: state === 'stale' ? value.ttlMs + 1000 : 200,
  };
}
export function instrumentReadings(state: ReadingState) {
  return {
    attitude: withState(live.attitude, state),
    heading: withState(live.heading, state),
    position: withState(live.position, state),
    flightPath: withState(live.flightPath, state),
    airspeed: withState(live.airspeed, state),
    battery: withState(live.battery, state),
    guidance: withState(live.guidance, state),
    home: withState(live.home, state),
    link: withState(live.link, state),
  };
}
