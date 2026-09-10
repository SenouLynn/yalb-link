/**
 * The onboard mission, in two panels that sit in two different columns.
 *
 * `MissionPanel` is progress: what is held, which waypoint the vehicle is
 * flying, and the lever that fetches it. It belongs beside the vehicle's other
 * state, because "which waypoint am I on" is a fact about the aircraft rather
 * than about the route.
 *
 * `WaypointList` is the route itself, and it belongs against the map that draws
 * it — the same information in two forms, read together. Splitting them is what
 * lets the operator keep the map and drop the list; before the split they were
 * one panel and the choice did not exist.
 *
 * The lever stays here rather than moving with the list. It acts on what this
 * panel reports, and this panel is in the column the operator cannot hide; a
 * download control that disappears with the waypoints would take the only way
 * of getting them back with it.
 */

import { useEffect, useState } from 'react';

import type { MissionItem, MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import { NO_VALUE, tidy } from '@/ui/format';
import { Row, Group, Chip, Note, Lever } from '@/ui/primitives';
import type { Reading } from '@/ui/readings';

import { commandName, frameName, type MissionGeometry } from './model';

export type MissionStatus = 'idle' | 'loading' | 'error' | 'complete';

export interface MissionPanelProps {
  nowMs?: number;
  status: MissionStatus;
  snapshot: MissionSnapshot | null;
  error: string | null;
  activeSeq: number | null;
  onDownload: () => void;
}

export function MissionPanel({
  status,
  snapshot,
  error,
  activeSeq,
  onDownload,
  nowMs = Date.now(),
}: MissionPanelProps) {
  /*
   * Nothing known about the route collapses to a sentence, not to a column of
   * dashes — the rule the consulted groups in the rail already follow. Before a
   * download and before the first MISSION_CURRENT there is no route state to
   * report, and two dashes under a line that already says "Not downloaded" is
   * the same absence stated three times.
   */
  const known = snapshot !== null || activeSeq !== null;

  return (
    <Group
      className="mission-panel"
      label="Mission"
      reading={missionReading(snapshot, nowMs)}
      aria-label="Onboard mission"
    >
      {known ? (
        <>
          <Row
            label="Current waypoint"
            value={activeLabel(snapshot, activeSeq)}
            tone={activeSeq === null ? 'dead' : 'normal'}
          />
          <Row
            label="Items"
            value={snapshot === null ? NO_VALUE : String(snapshot.items.length)}
            tone={snapshot === null ? 'dead' : 'normal'}
          />
        </>
      ) : null}

      {/* Status prose above the lever, never beside it: a control and its
          status read as one thing stacked and as two things side by side. */}
      {statusLabel(status, snapshot) === null ? null : (
        <Note role="status">{statusLabel(status, snapshot)}</Note>
      )}
      {error === null ? null : (
        <Note role="alert" tone="caution">
          {error}
        </Note>
      )}
      <Lever wide disabled={status === 'loading'} onClick={onDownload}>
        {status === 'loading' ? 'Downloading…' : snapshot === null ? 'Download mission' : 'Refresh mission'}
      </Lever>
    </Group>
  );
}

/**
 * The commanded route, as the list the map draws.
 *
 * Every item is collapsed except the one the vehicle is flying to. A sixty-item
 * survey grid expanded is eight rows apiece — four hundred rows in a column
 * that also has to hold the map, which is a list nobody reads. Collapsed, the
 * route reads as a route: one line per leg, with the leg in progress open.
 *
 * The operator's own expansions win and persist, and the active item still
 * opens and closes on its own around them — so watching the vehicle work
 * through a mission needs no clicks, and inspecting a leg ahead does not get
 * undone the moment the vehicle reaches the next waypoint.
 */
export function WaypointList({
  snapshot,
  geometry,
  activeSeq,
  nowMs = Date.now(),
}: {
  nowMs?: number;
  snapshot: MissionSnapshot | null;
  geometry: MissionGeometry;
  activeSeq: number | null;
}) {
  /* Per-item overrides on top of "the active one is open". Cleared when the
     snapshot changes: the seq numbers in them describe the old route. */
  const [opened, setOpened] = useState<Readonly<Record<number, boolean>>>({});

  useEffect(() => {
    setOpened({});
  }, [snapshot]);

  return (
    <Group
      className="mission-panel"
      label="Waypoints"
      reading={missionReading(snapshot, nowMs)}
      aria-label="Mission waypoints"
      absent={absentReason(snapshot)}
    >
      <ol className="mission-list">
        {(snapshot?.items ?? []).map((item) => {
          const active = activeSeq === item.seq;

          return (
            <li key={item.seq} className={active ? 'mission-list__active' : undefined}>
              <details
                open={opened[item.seq] ?? active}
                onToggle={(event) => {
                  const isOpen = event.currentTarget.open;
                  setOpened((previous) => ({ ...previous, [item.seq]: isOpen }));
                }}
              >
                <summary className="group__head group__head--disclosure">
                  <span className="group__label">
                    #{item.seq} {commandName(item.command)}
                  </span>
                  {active ? <Chip tone="active">Active</Chip> : null}
                  {/* The altitude, because it is what an operator scans a
                      folded route for and it is otherwise three clicks deep. */}
                  <span className="group__annotation">{tidy(item.z)}</span>
                </summary>
                <Row label="Frame" value={frameName(item.frame)} />
                {/* One row. The pair is x/y rather than a coordinate in a
                    local frame, which the Frame row above already states and
                    the pointer repeats. */}
                <Row
                  label="Lat / lon"
                  hint="x / y in a local frame"
                  value={`${tidy(item.x)}, ${tidy(item.y)}`}
                />
                <Row label="Alt / Z" value={tidy(item.z)} />
                {itemParams(item).map(([label, value]) => (
                  <Row key={label} label={label} value={value} />
                ))}
                <Row label="Autocontinue" value={item.autocontinue ? 'yes' : 'no'} />
                {geometry.omitted[item.seq] === undefined ? null : (
                  <Note tone="caution">Not mapped: {geometry.omitted[item.seq]}</Note>
                )}
              </details>
            </li>
          );
        })}
      </ol>
    </Group>
  );
}

/**
 * Both panels state the same provenance, because they render the same snapshot.
 *
 * A snapshot identifies a non-expiring observation and not proof that the
 * onboard mission is unchanged, which is why it never goes stale on a TTL.
 */
function missionReading(snapshot: MissionSnapshot | null, nowMs: number): Reading<unknown> | undefined {
  const observed = snapshot?.observedAt;

  if (observed === undefined) {
    return undefined;
  }

  return {
    state: 'live',
    value: snapshot,
    source: 'MISSION SNAPSHOT',
    ageMs: Math.max(0, nowMs - (Number(observed.seconds) * 1000 + observed.nanos / 1e6)),
    ttlMs: Infinity,
  };
}

/** Why there is no list, in the operator's terms rather than as an empty box. */
function absentReason(snapshot: MissionSnapshot | null): string | undefined {
  if (snapshot === null) {
    return 'No mission downloaded. Fetch one from the Mission panel.';
  }

  return snapshot.items.length === 0 ? 'Vehicle reported an empty mission.' : undefined;
}

/** The waypoint being flown, named the way the list names it. */
function activeLabel(snapshot: MissionSnapshot | null, activeSeq: number | null): string {
  if (activeSeq === null) {
    return NO_VALUE;
  }

  const item = snapshot?.items.find((candidate) => candidate.seq === activeSeq);

  // The sequence is reported by MISSION_CURRENT and the name by the downloaded
  // snapshot, so the number is knowable while the name is not. Showing the bare
  // number is honest; withholding it because the mission has not been fetched
  // would hide the one piece of route state that arrives on its own.
  return item === undefined ? `#${String(activeSeq)}` : `#${String(activeSeq)} ${commandName(item.command)}`;
}

/*
 * What is currently held, not what is in flight.
 *
 * The lever below says "Downloading…" while a download runs; a status line
 * saying it a second time beside it is the same sentence printed twice. And a
 * held snapshot needs no line at all — the rows above state the item count and
 * the waypoint being flown, which is what "complete" was standing in for.
 */
function statusLabel(status: MissionStatus, snapshot: MissionSnapshot | null): string | null {
  if (status === 'error') return 'Download failed — no current snapshot';
  if (status === 'loading') return 'Downloading…';
  if (snapshot === null) return 'Not downloaded';
  return null;
}

/**
 * The command parameters that carry information, as labelled rows.
 *
 * Named `param1`..`param4` rather than by meaning: the parameters are
 * command-specific and this display does not claim to decode them, the same
 * reason the flight mode is shown as a number. Zeroes are dropped — for most
 * commands an unset parameter says nothing, and four of them in a row is the
 * wire dump this panel exists to avoid.
 */
function itemParams(item: MissionItem): [string, string][] {
  return ([
    ['Param 1', item.param1],
    ['Param 2', item.param2],
    ['Param 3', item.param3],
    ['Param 4', item.param4],
  ] as const)
    .filter(([, value]) => value !== 0)
    .map(([label, value]) => [label, String(value)]);
}
