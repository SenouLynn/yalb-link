import type { MissionItem, MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import { MavCmd, MavFrame } from '@/gen/gcs/v1/types_pb';
import type { GeoCoordinate } from '@/logic/trajectory';
import { isFamilyFresh, type VehicleView } from '@/fleet/state';

export interface MissionGeometry {
  points: (GeoCoordinate & { seq: number })[];
  omitted: Readonly<Record<number, string>>;
}

const globalFrames = new Set<MavFrame>([
  MavFrame.GLOBAL,
  MavFrame.GLOBAL_RELATIVE_ALT,
  MavFrame.GLOBAL_INT,
  MavFrame.GLOBAL_RELATIVE_ALT_INT,
]);

const positionalCommands = new Set<MavCmd>([
  MavCmd.NAV_WAYPOINT,
  MavCmd.NAV_LOITER_UNLIM,
  MavCmd.NAV_LOITER_TURNS,
  MavCmd.NAV_LOITER_TIME,
  MavCmd.NAV_LAND,
  MavCmd.NAV_TAKEOFF,
  MavCmd.NAV_LOITER_TO_ALT,
]);

export function classifyItem(item: MissionItem): string | null {
  if (!globalFrames.has(item.frame)) return `frame ${frameName(item.frame)} is not supported on the map`;
  if (!positionalCommands.has(item.command)) return `command ${commandName(item.command)} is non-positional`;
  if (!Number.isFinite(item.x) || !Number.isFinite(item.y)) return 'coordinates are not finite';
  return null;
}

export function missionGeometry(snapshot: MissionSnapshot | null): MissionGeometry {
  const points: MissionGeometry['points'] = [];
  const omitted: Record<number, string> = {};
  for (const item of snapshot?.items ?? []) {
    const reason = classifyItem(item);
    if (reason === null) points.push({ latDeg: item.x, lonDeg: item.y, seq: item.seq });
    else omitted[item.seq] = reason;
  }
  return { points, omitted };
}

export function commandName(value: MavCmd): string {
  return MavCmd[value];
}

export function frameName(value: MavFrame): string {
  return MavFrame[value];
}

export function activeMissionSequence(view: VehicleView, nowMs: number): number | null {
  return isFamilyFresh(view, 'MISSION_CURRENT', nowMs) ? view.sample.missionCurrentSeq ?? null : null;
}
