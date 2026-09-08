/**
 * A deterministic mission for the no-backend display.
 *
 * The mission panel is the one surface that cannot be demonstrated from the
 * stream fixtures, because a mission arrives over a synchronous HTTP read
 * rather than the event stream. This snapshot is the same protobuf message the
 * backend serves, built by hand for the same reason `stream/fixtures.ts`
 * exists: so the panel can be looked at, and a rendering defect reproduced,
 * without SITL and without an external ground station to load a mission.
 *
 * The route sits on the mock vehicle's own position so the commanded line, the
 * flown track, and the prediction are all visible together — which is the
 * comparison the display exists to make.
 */

import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';

import { MissionItemSchema, MissionSnapshotSchema, type MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import { MavCmd, MavFrame, MavMissionType } from '@/gen/gcs/v1/types_pb';
import { VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';

/** Anchored on the mock vehicle's starting position. */
const HOME_LAT = 37.7749;
const HOME_LON = -122.4194;

/**
 * Items 2 and 5 carry MAV_FRAME_MISSION and non-positional commands, so they
 * stay in the list and out of the map geometry. A fixture with only mappable
 * waypoints would never show the "Not mapped" explanation, which is a posture
 * the display is specifically required to have.
 */
const items = [
  { seq: 0, frame: MavFrame.GLOBAL_RELATIVE_ALT_INT, command: MavCmd.NAV_TAKEOFF, x: HOME_LAT, y: HOME_LON, z: 25, param1: 15 },
  { seq: 1, frame: MavFrame.GLOBAL_RELATIVE_ALT_INT, command: MavCmd.NAV_WAYPOINT, x: HOME_LAT + 0.0020, y: HOME_LON, z: 40 },
  { seq: 2, frame: MavFrame.MISSION, command: MavCmd.DO_CHANGE_SPEED, x: 0, y: 0, z: 0, param2: 12 },
  { seq: 3, frame: MavFrame.GLOBAL_RELATIVE_ALT_INT, command: MavCmd.NAV_WAYPOINT, x: HOME_LAT + 0.0020, y: HOME_LON + 0.0040, z: 40 },
  { seq: 4, frame: MavFrame.GLOBAL_RELATIVE_ALT_INT, command: MavCmd.NAV_LOITER_TURNS, x: HOME_LAT, y: HOME_LON + 0.0040, z: 40, param1: 2 },
  { seq: 5, frame: MavFrame.MISSION, command: MavCmd.NAV_RETURN_TO_LAUNCH, x: 0, y: 0, z: 0 },
];

/** Total items in the fixture, so the mock stream can walk the active index. */
export const MOCK_MISSION_LENGTH = items.length;

export function mockMissionSnapshot(
  systemId: number,
  componentId: number,
  observedAtMs: number = Date.now(),
): MissionSnapshot {
  return create(MissionSnapshotSchema, {
    vehicleId: create(VehicleIdSchema, { systemId, componentId }),
    missionType: MavMissionType.MISSION,
    items: items.map((item) => create(MissionItemSchema, { ...item, autocontinue: true })),
    observedAt: timestampFromMs(observedAtMs),
  });
}
