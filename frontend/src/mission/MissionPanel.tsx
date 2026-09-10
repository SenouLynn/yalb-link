import type { MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import { Row, Group, Chip, Note, Lever } from '@/ui/primitives';
import { commandName, frameName, type MissionGeometry } from './model';
import type { MissionItem } from '@/gen/gcs/v1/missions_pb';

export type MissionStatus = 'idle' | 'loading' | 'error' | 'complete';

export function MissionPanel({ status, snapshot, error, geometry, activeSeq, onDownload, nowMs = Date.now() }: {
  nowMs?: number;
  status: MissionStatus;
  snapshot: MissionSnapshot | null;
  error: string | null;
  geometry: MissionGeometry;
  activeSeq: number | null;
  onDownload: () => void;
}) {
  const observed = snapshot?.observedAt;
  const reading = observed === undefined ? undefined : {
    state: 'live' as const, value: snapshot, source: 'MISSION SNAPSHOT',
    ageMs: Math.max(0, nowMs - (Number(observed.seconds) * 1000 + observed.nanos / 1e6)),
    ttlMs: Infinity,
  };
  return <Group className="mission-panel" label="Mission" reading={reading} aria-label="Onboard mission">
    <div className="mission-panel__header">
      <span className="mission-panel__state">{statusLabel(status, snapshot)}</span>
      <Lever disabled={status === 'loading'} onClick={onDownload}>
        {status === 'loading' ? 'Downloading…' : snapshot === null ? 'Download mission' : 'Refresh mission'}
      </Lever>
    </div>
    {error === null ? null : <Note role="alert" tone="caution">{error}</Note>}
    {snapshot?.items.length === 0 ? <Note tone="absent">Vehicle reported an empty mission.</Note> : null}
    {snapshot === null || snapshot.items.length === 0 ? null : <ol className="mission-list">
      {snapshot.items.map((item) => <li key={item.seq} className={activeSeq === item.seq ? 'mission-list__active' : undefined}>
        <div className="mission-list__head"><span>#{item.seq} {commandName(item.command)}</span>
          {activeSeq === item.seq ? <Chip tone="active">Active</Chip> : null}</div>
        <Row label="Frame" value={frameName(item.frame)} />
        <Row label="Lat / X" value={String(item.x)} />
        <Row label="Lon / Y" value={String(item.y)} />
        <Row label="Alt / Z" value={String(item.z)} />
        {itemParams(item).map(([label, value]) => <Row key={label} label={label} value={value} />)}
        <Row label="Autocontinue" value={item.autocontinue ? 'yes' : 'no'} />
        {geometry.omitted[item.seq] === undefined ? null : <Note tone="caution">Not mapped: {geometry.omitted[item.seq]}</Note>}
      </li>)}
    </ol>}
  </Group>;
}

function statusLabel(status: MissionStatus, snapshot: MissionSnapshot | null): string {
  if (status === 'idle') return 'Not downloaded';
  if (status === 'loading') return 'Loading current vehicle…';
  if (status === 'error') return 'Download failed — no current snapshot';
  return snapshot?.items.length === 0 ? 'Complete · empty' : `Complete · ${String(snapshot?.items.length ?? 0)} items`;
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
