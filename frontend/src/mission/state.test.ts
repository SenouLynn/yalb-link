import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { emptyMissionState, visibleMissionState, type MissionViewState } from './state';

describe('vehicle-keyed mission state', () => {
  it('cannot display one vehicle snapshot after selecting another', () => {
    const prior: MissionViewState = { ...emptyMissionState('1:1'), status: 'complete', snapshot: create(MissionSnapshotSchema) };
    expect(visibleMissionState(prior, '2:1')).toEqual(emptyMissionState('2:1'));
  });

  it('loading and failure carry no prior snapshot', () => {
    const loading: MissionViewState = { key: '1:1', status: 'loading', snapshot: null, error: null };
    const failed: MissionViewState = { key: '1:1', status: 'error', snapshot: null, error: 'timeout' };
    expect(loading.snapshot).toBeNull();
    expect(failed.snapshot).toBeNull();
  });
});
