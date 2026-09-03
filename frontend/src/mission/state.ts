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
