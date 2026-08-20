/** The assembled mini flight display for one selected vehicle. */

import type { FleetState, VehicleKey } from '@/fleet/state';
import { MapPanel } from '@/map/MapPanel';

import { AttitudeIndicator } from './AttitudeIndicator';
import { HeadingIndicator } from './HeadingIndicator';
import { NO_VALUE, num, signed } from './format';
import { hasDisplayValue, readFlight } from './readings';
import { Readout } from './Readout';
import { StatusBar } from './StatusBar';
import { VehicleSelector } from './VehicleSelector';

export interface FlightDisplayProps {
  fleet: FleetState;
  /** Injected clock; freshness is measured against it. */
  nowMs: number;
  mock: boolean;
  onSelect: (key: VehicleKey) => void;
}

export function FlightDisplay({ fleet, nowMs, mock, onSelect }: FlightDisplayProps) {
  const view = fleet.selected === null ? undefined : fleet.vehicles[fleet.selected];

  if (view === undefined) {
    return <EmptyFleet connected={fleet.connected} mock={mock} />;
  }

  const readings = readFlight(view, nowMs);
  const { position, flightPath, battery } = readings;

  const altitude = hasDisplayValue(position) ? position.value : null;
  const path = hasDisplayValue(flightPath) ? flightPath.value : null;
  const power = hasDisplayValue(battery) ? battery.value : null;

  return (
    <div className="display">
      <StatusBar view={view} nowMs={nowMs} connected={fleet.connected} mock={mock} />

      <VehicleSelector fleet={fleet} onSelect={onSelect} />

      <div className="instruments">
        <AttitudeIndicator reading={readings.attitude} />

        <div className="readouts">
          <Readout
            label="Altitude"
            note={altitude === null ? 'datum unknown' : datumNote(altitude.altRef)}
            value={altitude === null ? NO_VALUE : num(altitude.altM)}
            unit="m"
            reading={position}
            wide
          />

          <Readout
            label="Ground speed"
            value={path === null ? NO_VALUE : num(path.groundSpeedMps)}
            unit="m/s"
            reading={flightPath}
          />

          <Readout
            label="Climb"
            value={path === null ? NO_VALUE : signed(path.climbMps)}
            unit="m/s"
            reading={flightPath}
          />

          <Readout
            label="Battery"
            value={power === null ? NO_VALUE : num(power.voltageV, 2)}
            unit="V"
            reading={battery}
          />

          <Readout
            label="Remaining"
            value={power === null ? NO_VALUE : num(power.remainingPct, 0)}
            unit="%"
            reading={battery}
          />
        </div>
      </div>

      <HeadingIndicator reading={readings.heading} />

      <MapPanel
        position={hasDisplayValue(readings.position) ? readings.position.value : null}
        track={view.track}
      />
    </div>
  );
}

/** Names the altitude datum explicitly; the two differ by field elevation. */
function datumNote(ref: 'RELATIVE' | 'MSL'): string {
  return ref === 'RELATIVE' ? 'above home' : 'above sea level';
}

function EmptyFleet({ connected, mock }: { connected: boolean; mock: boolean }) {
  return (
    <div className="display">
      <div className="panel empty">
        <div className="label">No vehicle</div>
        <p className="empty__hint">
          {mock
            ? 'Replaying fixtures. The mock vehicle appears on the first frame.'
            : connected
              ? 'Connected to the backend. Waiting for a heartbeat.'
              : 'Not connected to the backend. Start it, or open ?source=mock.'}
        </p>
      </div>
    </div>
  );
}
