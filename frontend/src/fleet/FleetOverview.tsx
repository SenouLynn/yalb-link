import { useState, type RefObject } from 'react';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { FleetState } from './state';
import { fleetPositions } from './overview';
import { FleetMap, type SavedCamera } from '@/map/FleetMap';
import { framePoints, type Frame } from '@/map/camera';
import { readFlight, hasDisplayValue } from '@/ui/readings';
import { num, age, NO_VALUE } from '@/ui/format';
import type { StreamSource } from '@/stream/select';

export function FleetOverview({ fleet, nowMs, onOpen, camera, source }: {
  fleet: FleetState; nowMs: number; onOpen: (key: string) => void;
  camera: RefObject<SavedCamera | null>; source: StreamSource;
}) {
  const positions = fleetPositions(fleet, nowMs);
  const [request, setRequest] = useState<{ frame: Frame } | null>(null);
  const fit = () => { const frame = framePoints(positions); if (frame) setRequest({ frame }); };
  return <>
    <aside className="workspace__sidebar fleet-roster" aria-label="Fleet roster">
      <div className="fleet-roster__heading"><h2>Fleet <span className="label">{fleet.order.length} vehicles</span></h2>
        <button type="button" onClick={fit} disabled={positions.length === 0}>Fit fleet</button></div>
      <p className="label">{source === 'live' ? (fleet.connected ? 'Backend connected' : 'Backend disconnected') : source === 'replay' ? 'Recorded fleet' : 'Fixture fleet'}</p>
      {fleet.order.length === 0 && <p className="empty__hint">Waiting for vehicles</p>}
      {fleet.order.map(key => {
        const view = fleet.vehicles[key];
        if (!view) return null;
        const readings = readFlight(view, nowMs);
        const point = positions.find(p => p.key === key);
        const position = hasDisplayValue(readings.position) ? readings.position.value : null;
        const path = hasDisplayValue(readings.flightPath) ? readings.flightPath.value : null;
        const lost = view.lifecycle === FleetEventType.VEHICLE_LOST;
        return <article className="fleet-card" key={key} aria-label={`Vehicle ${key}`} data-selected={fleet.selected === key}>
          <div className="fleet-card__title"><strong>{key}</strong><span className={`chip chip--${lost ? 'caution' : 'active'}`}>{lost ? 'LOST' : 'HEARD'}</span></div>
          <p className="fleet-card__position">{point ? `${point.latDeg.toFixed(6)}, ${point.lonDeg.toFixed(6)}` : 'Position unavailable'}</p>
          <dl><div><dt>Altitude</dt><dd>{position && !lost ? `${num(position.altM)} m · ${position.altRef === 'RELATIVE' ? 'above home' : 'MSL'}` : NO_VALUE}</dd></div>
            <div><dt>Ground speed</dt><dd>{path && !lost ? `${num(path.groundSpeedMps)} m/s` : NO_VALUE}</dd></div>
            <div><dt>Position age</dt><dd>{age(readings.position.ageMs)} · {lost ? 'lost' : readings.position.state}</dd></div></dl>
          <div className="fleet-card__actions">
            <button type="button" aria-label={`Open vehicle ${key}`} onClick={() => { onOpen(key); }}>Open</button>
            <button type="button" aria-label={`Center vehicle ${key}`} disabled={!point} onClick={() => { if (point) { const frame = framePoints([point]); if (frame) setRequest({ frame }); } }}>Center</button>
          </div>
        </article>;
      })}
    </aside>
    <main className="workspace__main fleet-main" aria-label="Fleet map"><FleetMap positions={positions} onOpen={onOpen} camera={camera} request={request} /></main>
  </>;
}
