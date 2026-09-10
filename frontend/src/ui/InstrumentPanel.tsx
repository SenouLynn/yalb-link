import { AttitudeIndicator } from './AttitudeIndicator';
import { HeadingIndicator } from './HeadingIndicator';
import { NO_VALUE, num, signed } from './format';
import { Group, Row } from './primitives';
import { hasDisplayValue, type FlightReadings } from './readings';

/**
 * The primary readings, grouped by the MAVLink family they resolve from.
 *
 * Grouping by source is what lets provenance be stated once per table instead
 * of once per reading. Three readings all sourced from VFR_HUD used to print
 * `VFR_HUD` and an age three times, each on its own right-aligned line at a
 * different edge from the values — which destroyed the shared value edge that
 * makes a column of readings scan as one table.
 *
 * The consulted tier (guidance, home, radio link) is not here: it lives in the
 * rail, because it is read before and after a flight rather than during one.
 */
export function InstrumentPanel({ readings }: { readings: FlightReadings }) {
  const { position, flightPath, airspeed, battery } = readings;
  const altitude = hasDisplayValue(position) ? position.value : null;
  const path = hasDisplayValue(flightPath) ? flightPath.value : null;
  const power = hasDisplayValue(battery) ? battery.value : null;
  const air = hasDisplayValue(airspeed) ? airspeed.value : null;

  return (
    <>
      <AttitudeIndicator reading={readings.attitude} />

      <Group label="Altitude" reading={position}>
        <Row
          label="Altitude"
          note={datumNote(altitude)}
          value={altitude === null ? NO_VALUE : num(altitude.altM)}
          unit="m"
          tone={altitude === null ? 'dead' : 'normal'}
          lead
        />
      </Group>

      <Group label="Flight path" reading={flightPath}>
        <Row
          label="Ground speed"
          value={path === null ? NO_VALUE : num(path.groundSpeedMps)}
          unit="m/s"
          tone={path === null ? 'dead' : 'normal'}
        />
        <Row
          label="Climb"
          value={path === null ? NO_VALUE : signed(path.climbMps)}
          unit="m/s"
          tone={path === null ? 'dead' : 'normal'}
        />
      </Group>

      <Group label="Airspeed" reading={airspeed}>
        <Row
          label="Airspeed"
          value={air === null ? NO_VALUE : num(air.airspeedMps)}
          unit="m/s"
          tone={air === null ? 'dead' : 'normal'}
        />
      </Group>

      <Group label="Power" reading={battery}>
        <Row
          label="Battery"
          value={power === null ? NO_VALUE : num(power.voltageV, 2)}
          unit="V"
          tone={power === null ? 'dead' : 'normal'}
        />
        <Row
          label="Remaining"
          value={power === null ? NO_VALUE : num(power.remainingPct, 0)}
          unit="%"
          tone={power === null ? 'dead' : 'normal'}
        />
      </Group>

      <HeadingIndicator reading={readings.heading} />
    </>
  );
}

/** Names the altitude datum explicitly; the two differ by field elevation. */
function datumNote(altitude: { altRef: 'RELATIVE' | 'MSL' } | null): string {
  if (altitude === null) {
    return 'datum unknown';
  }

  return altitude.altRef === 'RELATIVE' ? 'above home' : 'above sea level';
}
