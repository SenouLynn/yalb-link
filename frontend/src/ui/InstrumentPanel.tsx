import { AttitudeIndicator } from './AttitudeIndicator';
import { HeadingIndicator } from './HeadingIndicator';
import { NO_VALUE, num, signed } from './format';
import { hasDisplayValue, readFlight } from './readings';
import { Readout } from './Readout';

/** Primary readings. T-015 inspection belongs below these in this pane's scroll body. */
export function InstrumentPanel({ readings }: { readings: ReturnType<typeof readFlight> }) {
  const { position, flightPath, battery } = readings;
  const altitude = hasDisplayValue(position) ? position.value : null;
  const path = hasDisplayValue(flightPath) ? flightPath.value : null;
  const power = hasDisplayValue(battery) ? battery.value : null;
  return <>
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

  </>;
}

/** Names the altitude datum explicitly; the two differ by field elevation. */
function datumNote(ref: 'RELATIVE' | 'MSL'): string {
  return ref === 'RELATIVE' ? 'above home' : 'above sea level';
}
