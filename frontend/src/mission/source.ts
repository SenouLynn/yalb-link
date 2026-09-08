/**
 * Chooses where a mission comes from.
 *
 * Only the live display talks to the backend. The mock display serves a
 * fixture, and does so behind this single switch so the fixture cannot leak
 * into a live session: a mission shown next to real telemetry is a commanded
 * route an operator may act on, and it has to have come from the vehicle.
 *
 * Replay deliberately uses the live loader. Recordings store stream events and
 * a mission snapshot was never one of them, so there is nothing recorded to
 * replay; asking the backend and reporting that it has no route to a vehicle
 * that is not flying is the honest answer.
 */

import type { MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import type { StreamSource } from '@/stream/select';

import { downloadMission } from './client';
import { mockMissionSnapshot } from './fixtures';

export type MissionLoader = (
  sysId: number,
  compId: number,
  signal?: AbortSignal,
) => Promise<MissionSnapshot>;

function loadMockMission(
  sysId: number,
  compId: number,
  signal?: AbortSignal,
): Promise<MissionSnapshot> {
  // The fixture is immediate, but cancellation still has to behave like the
  // network path or the mock would not exercise the same state machine.
  if (signal?.aborted === true) {
    return Promise.reject(new DOMException('mission download aborted', 'AbortError'));
  }

  return Promise.resolve(mockMissionSnapshot(sysId, compId));
}

export function missionLoaderFor(source: StreamSource): MissionLoader {
  return source === 'mock' ? loadMockMission : downloadMission;
}
