import { AttitudeIndicator } from './AttitudeIndicator';
import { HeadingIndicator } from './HeadingIndicator';
import { age, NO_VALUE, num, signed } from './format';
import { Group, Row } from './primitives';
import { hasDisplayValue, type FlightReadings, type Reading } from './readings';

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
 *
 * Altitude, flight path, and airspeed collapse into one "Flight data" pane —
 * styled like every other rail accordion — rather than three separate groups.
 * One header can only carry one family's provenance faithfully, though, and
 * these three are three different families with three different ages.
 * Altitude keeps the header's own drain bar, since it's already this group's
 * `lead` row; ground speed/climb and airspeed name their own source and age on
 * their first row's hint instead, the same way `HomeRows` states a source that
 * differs from its group header's.
 */
export function InstrumentPanel({ readings }: { readings: FlightReadings }) {
  const { position, flightPath, airspeed } = readings;
  const altitude = hasDisplayValue(position) ? position.value : null;
  const path = hasDisplayValue(flightPath) ? flightPath.value : null;
  const air = hasDisplayValue(airspeed) ? airspeed.value : null;

  return (
    <>
      <HeadingIndicator reading={readings.heading} />
      <AttitudeIndicator reading={readings.attitude} />

      <Group label="Flight data" reading={position} collapsible>
        <Row
          label="Altitude"
          hint={datumNote(altitude)}
          value={altitude === null ? NO_VALUE : num(altitude.altM)}
          unit="m"
          tone={altitude === null ? 'dead' : 'normal'}
          lead
        />
        <Row
          label="Ground speed"
          hint={sourceHint(flightPath)}
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
        <Row
          label="Airspeed"
          hint={sourceHint(airspeed)}
          value={air === null ? NO_VALUE : num(air.airspeedMps)}
          unit="m/s"
          tone={air === null ? 'dead' : 'normal'}
        />
      </Group>
    </>
  );
}

/** Names a reading's own source and age, for a row whose family the group
 *  header does not already state — see `HomeRows` for the same pattern. */
function sourceHint(reading: Reading<unknown>): string | undefined {
  if (reading.source === null || reading.ageMs === null) {
    return undefined;
  }

  return `${reading.source} · ${age(reading.ageMs)}`;
}

/** Names the altitude datum explicitly; the two differ by field elevation. */
function datumNote(altitude: { altRef: 'RELATIVE' | 'MSL' } | null): string {
  if (altitude === null) {
    return 'datum unknown';
  }

  return altitude.altRef === 'RELATIVE' ? 'above home' : 'above sea level';
}
