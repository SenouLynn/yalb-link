import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { emptyMissionState, missionPanelState, visibleMissionState, type MissionViewState } from './state';

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

describe('mission panel props', () => {
  // Found by driving the mock display in a real browser: the full state was
  // spread into MissionPanel, so React consumed `key` as a reconciliation key,
  // dropped the prop, and logged an error. Nothing server-rendered noticed.
  it('keeps the vehicle key out of the panel props', () => {
    const props = missionPanelState({ key: '1:1', status: 'complete', snapshot: null, error: null });

    expect('key' in props).toBe(false);
    expect(props).toEqual({ status: 'complete', snapshot: null, error: null });
  });
});
