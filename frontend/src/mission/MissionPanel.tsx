import type { MissionSnapshot } from '@/gen/gcs/v1/missions_pb';
import { commandName, frameName, type MissionGeometry } from './model';

export type MissionStatus = 'idle' | 'loading' | 'error' | 'complete';

export function MissionPanel({ status, snapshot, error, geometry, activeSeq, onDownload }: {
  status: MissionStatus;
  snapshot: MissionSnapshot | null;
  error: string | null;
  geometry: MissionGeometry;
  activeSeq: number | null;
  onDownload: () => void;
}) {
  return <section className="panel mission-panel" aria-label="Onboard mission">
    <div className="mission-panel__header">
      <div><div className="label">Onboard mission</div><div className="mission-panel__state">{statusLabel(status, snapshot)}</div></div>
      <button type="button" disabled={status === 'loading'} onClick={onDownload}>
        {status === 'loading' ? 'Downloading…' : snapshot === null ? 'Download mission' : 'Refresh mission'}
      </button>
    </div>
    {error === null ? null : <p role="alert" className="mission-panel__error">{error}</p>}
    {snapshot?.items.length === 0 ? <p className="mission-panel__empty">Vehicle reported an empty mission.</p> : null}
    {snapshot === null || snapshot.items.length === 0 ? null : <ol className="mission-list">
      {snapshot.items.map((item) => <li key={item.seq} className={activeSeq === item.seq ? 'mission-list__active' : undefined}>
        <div><strong>#{item.seq} {commandName(item.command)}</strong>{activeSeq === item.seq ? <span className="chip chip--active">Active</span> : null}</div>
        <div>frame {frameName(item.frame)} · lat/x {item.x} · lon/y {item.y} · alt/z {item.z}</div>
        <div>params [{item.param1}, {item.param2}, {item.param3}, {item.param4}] · autocontinue {item.autocontinue ? 'yes' : 'no'}</div>
        {geometry.omitted[item.seq] === undefined ? null : <div className="mission-list__note">Not mapped: {geometry.omitted[item.seq]}</div>}
      </li>)}
    </ol>}
  </section>;
}

function statusLabel(status: MissionStatus, snapshot: MissionSnapshot | null): string {
  if (status === 'idle') return 'Not downloaded';
  if (status === 'loading') return 'Loading current vehicle…';
  if (status === 'error') return 'Download failed — no current snapshot';
  return snapshot?.items.length === 0 ? 'Complete · empty' : `Complete · ${String(snapshot?.items.length ?? 0)} items`;
}
