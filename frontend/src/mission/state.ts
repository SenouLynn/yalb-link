import type { VehicleKey } from '@/fleet/state';
import type { MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import type { MissionStatus } from './MissionPanel';

export interface MissionViewState {
  key: VehicleKey;
  status: MissionStatus;
  snapshot: MissionSnapshot | null;
  error: string | null;
}

export function emptyMissionState(key: VehicleKey): MissionViewState {
  return { key, status: 'idle', snapshot: null, error: null };
}

/** Prevents a render after selection from borrowing the previous vehicle's data. */
export function visibleMissionState(state: MissionViewState, key: VehicleKey): MissionViewState {
  return state.key === key ? state : emptyMissionState(key);
}

/**
 * Narrows mission state to what the panel renders.
 *
 * `key` is dropped deliberately. It is a `VehicleKey`, and React treats a
 * `key` prop as a reconciliation key rather than passing it through, so
 * spreading the full state into a component silently loses it and logs an
 * error. Keeping the panel's props free of `key` makes that unrepresentable
 * rather than something each call site has to remember.
 */
export function missionPanelState({ status, snapshot, error }: MissionViewState) {
  return { status, snapshot, error };
}
