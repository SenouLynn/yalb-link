import { useState, type RefObject } from 'react';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { FleetState } from './state';
import { fleetPositions } from './overview';
import { FleetMap, type SavedCamera } from '@/map/FleetMap';
import { framePoints, type Frame } from '@/map/camera';
import { readFlight, hasDisplayValue } from '@/ui/readings';
import { num, age, NO_VALUE, sourceLabel } from '@/ui/format';
import { Row, Group, Chip, Note, Lever } from '@/ui/primitives';
import type { StreamSource } from '@/stream/select';

export function FleetOverview({ fleet, nowMs, onOpen, camera, source }: {
  fleet: FleetState; nowMs: number; onOpen: (key: string) => void;
  camera: RefObject<SavedCamera | null>; source: StreamSource;
}) {
  const positions = fleetPositions(fleet, nowMs);
  const [request, setRequest] = useState<{ frame: Frame } | null>(null);
  const fit = () => { const frame = framePoints(positions); if (frame) setRequest({ frame }); };
  return <main className="shell__body">
    <aside className="slot slot--rail fleet-roster" aria-label="Fleet roster">
      <Group label="Fleet" annotation={`${String(fleet.order.length)} vehicles`} actions={<Lever onClick={fit} disabled={positions.length === 0}>Fit fleet</Lever>}>
      <Row label="Source" value={sourceLabel(source, fleet.connected)}
        tone={source === 'live' && fleet.connected ? 'normal' : 'caution'} />
      {fleet.order.length === 0 && <Note tone="absent">Waiting for vehicles. Nothing has reported on the link yet.</Note>}
      {fleet.order.map(key => {
        const view = fleet.vehicles[key];
        if (!view) return null;
        const readings = readFlight(view, nowMs);
        const point = positions.find(p => p.key === key);
        const position = hasDisplayValue(readings.position) ? readings.position.value : null;
        const path = hasDisplayValue(readings.flightPath) ? readings.flightPath.value : null;
        const lost = view.lifecycle === FleetEventType.VEHICLE_LOST;
        return <article className="fleet-card" key={key} aria-label={`Vehicle ${key}`} data-selected={fleet.selected === key}>
          <div className="fleet-card__title"><strong>{key}</strong><Chip tone={lost ? 'caution' : 'active'}>{lost ? 'LOST' : 'HEARD'}</Chip></div>
          <p className="fleet-card__position">{point ? `${point.latDeg.toFixed(6)}, ${point.lonDeg.toFixed(6)}` : 'Position unavailable'}</p>
          <dl><div><dt>Altitude</dt><dd>{position && !lost ? `${num(position.altM)} m · ${position.altRef === 'RELATIVE' ? 'above home' : 'MSL'}` : NO_VALUE}</dd></div>
            <div><dt>Ground speed</dt><dd>{path && !lost ? `${num(path.groundSpeedMps)} m/s` : NO_VALUE}</dd></div>
            <div><dt>Position age</dt><dd>{age(readings.position.ageMs)} · {lost ? 'lost' : readings.position.state}</dd></div></dl>
          <div className="fleet-card__actions">
            <Lever aria-label={`Open vehicle ${key}`} onClick={() => { onOpen(key); }}>Open</Lever>
            <Lever aria-label={`Center vehicle ${key}`} disabled={!point} onClick={() => { if (point) { const frame = framePoints([point]); if (frame) setRequest({ frame }); } }}>Center</Lever>
          </div>
        </article>;
      })}
    </Group></aside>
    <section className="slot slot--center fleet-main" aria-label="Fleet map"><FleetMap positions={positions} onOpen={onOpen} camera={camera} request={request} /></section>
  </main>;
}
