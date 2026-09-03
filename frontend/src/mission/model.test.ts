import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { MissionItemSchema, MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { MavCmd, MavFrame } from '@/gen/gcs/v1/types_pb';
import type { VehicleView } from '@/fleet/state';
import { TELEMETRY_TTL_MS } from '@/fleet/state';
import { activeMissionSequence, missionGeometry } from './model';

describe('mission geometry', () => {
  it('keeps ordered supported positions and explains every omission', () => {
    const snapshot = create(MissionSnapshotSchema, { items: [
      create(MissionItemSchema, { seq: 0, frame: MavFrame.GLOBAL_RELATIVE_ALT_INT, command: MavCmd.NAV_WAYPOINT, x: 47.1, y: -122.1, z: 40 }),
      create(MissionItemSchema, { seq: 1, frame: MavFrame.MISSION, command: MavCmd.DO_SET_SERVO, x: 9, y: 10 }),
      create(MissionItemSchema, { seq: 2, frame: MavFrame.GLOBAL_INT, command: MavCmd.NAV_LAND, x: 47.2, y: -122.2, z: 0 }),
    ] });
    const result = missionGeometry(snapshot);
    expect(result.points.map((point) => point.seq)).toEqual([0, 2]);
    expect(result.omitted[1]).toContain('not supported');
  });
});

describe('active mission sequence', () => {
  const view: VehicleView = {
    key: '1:1', sysId: 1, compId: 1, lifecycle: undefined, heartbeat: undefined,
    lastFleetAtMs: undefined, lastSeenMs: 100, track: [],
    sample: { sourceMessage: 'MISSION_CURRENT', receivedAtMs: 100, missionCurrentSeq: 3 },
    familySeenMs: { MISSION_CURRENT: 100 },
  };
  it('uses only fresh MISSION_CURRENT state', () => {
    expect(activeMissionSequence(view, 100)).toBe(3);
    expect(activeMissionSequence(view, 100 + TELEMETRY_TTL_MS)).toBeNull();
    expect(activeMissionSequence({ ...view, familySeenMs: {} }, 100)).toBeNull();
  });
});
