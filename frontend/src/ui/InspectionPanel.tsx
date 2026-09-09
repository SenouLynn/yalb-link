/**
 * The detailed tier below the primary readings: guidance, home, and link.
 *
 * These three families are decoded by the backend and were, until now, dropped
 * by the browser. They sit below the instruments rather than beside them
 * because none of them is flown by — they are consulted. The pane body scrolls
 * independently, so depth here costs the primary readings nothing.
 */

import { bearing, num, signed, NO_VALUE } from './format';
import { hasDisplayValue } from './readings';
import type { FlightReadings } from './readings';
import { Readout } from './Readout';
import type { Reading } from './readings';
import type { ReactNode } from 'react';

export function InspectionPanel({ readings }: { readings: FlightReadings }) {
  return (
    <div className="inspection">
      <Group
        title="Guidance"
        reading={readings.guidance}
        absence="Not received. The autopilot reports it while it is navigating."
      >
        <GuidanceReadouts readings={readings} />
      </Group>

      <Group
        title="Home"
        reading={readings.home}
        absence="Not received. ArduPilot sends home when home is set, not on an interval."
      >
        <HomeReadouts readings={readings} />
      </Group>

      <Group
        title="Radio link"
        reading={readings.link}
        absence="Not received. Radio telemetry may be unavailable on this link."
      >
        <LinkReadouts readings={readings} />
      </Group>
    </div>
  );
}

/**
 * One titled group.
 *
 * A family that has never been heard collapses to a sentence rather than to a
 * column of dashes. The two conditions are genuinely different: a dash means a
 * value stopped arriving and the operator should notice, while RADIO_STATUS on
 * a UDP link is not late — it is not part of this system. Spending six dashes
 * on that would teach the operator to read dashes as decoration.
 *
 * Stale is not absent, and stays with the readouts: a value inside a group that
 * has gone stale still renders as `- - -` with its age, which is the posture
 * every other instrument on the display already takes.
 */
function Group({
  title,
  reading,
  absence,
  children,
}: {
  title: string;
  reading: Reading<unknown>;
  absence: string;
  children: ReactNode;
}) {
  return (
    <section className="inspection__group" aria-label={title}>
      <h3 className="label inspection__title">{title}</h3>
      {reading.state === 'unavailable' ? (
        <p className="inspection__absent">{absence}</p>
      ) : (
        <div className="readouts">{children}</div>
      )}
    </section>
  );
}

function GuidanceReadouts({ readings }: { readings: FlightReadings }) {
  const { guidance } = readings;
  const value = hasDisplayValue(guidance) ? guidance.value : null;

  return (
    <>
      <Readout
        label="Target bearing"
        note="to waypoint"
        value={value === null ? NO_VALUE : bearing(value.targetBearingDeg)}
        unit="°"
        reading={guidance}
      />
      <Readout
        label="Nav bearing"
        note="commanded"
        value={value === null ? NO_VALUE : bearing(value.navBearingDeg)}
        unit="°"
        reading={guidance}
      />
      <Readout
        label="Waypoint distance"
        value={value === null ? NO_VALUE : num(value.wpDistM, 0)}
        unit="m"
        reading={guidance}
      />
      <Readout
        label="Crosstrack"
        note="+ right of leg"
        value={value === null ? NO_VALUE : signed(value.xtrackErrorM)}
        unit="m"
        reading={guidance}
      />
      <Readout
        label="Altitude error"
        note="+ below target"
        value={value === null ? NO_VALUE : signed(value.altErrorM)}
        unit="m"
        reading={guidance}
      />
      <Readout
        label="Airspeed error"
        note="+ below target"
        value={value === null ? NO_VALUE : signed(value.aspdErrorMps)}
        unit="m/s"
        reading={guidance}
      />
    </>
  );
}

function HomeReadouts({ readings }: { readings: FlightReadings }) {
  const { home } = readings;
  const value = hasDisplayValue(home) ? home.value : null;

  return (
    <>
      <Readout
        label="Home position"
        value={
          value === null
            ? NO_VALUE
            : `${value.latDeg.toFixed(6)}, ${value.lonDeg.toFixed(6)}`
        }
        reading={home}
        wide
      />
      <Readout
        label="Home elevation"
        note="above sea level"
        value={value === null ? NO_VALUE : num(value.altMslM)}
        unit="m"
        reading={home}
      />
    </>
  );
}

function LinkReadouts({ readings }: { readings: FlightReadings }) {
  const { link } = readings;
  const value = hasDisplayValue(link) ? link.value : null;

  return (
    <>
      <Readout
        label="Signal"
        note="local"
        value={value === null ? NO_VALUE : num(value.rssi, 0)}
        reading={link}
      />
      <Readout
        label="Signal"
        note="remote"
        value={value === null ? NO_VALUE : num(value.remrssi, 0)}
        reading={link}
      />
      <Readout
        label="Noise"
        note="local"
        value={value === null ? NO_VALUE : num(value.noise, 0)}
        reading={link}
      />
      <Readout
        label="Noise"
        note="remote"
        value={value === null ? NO_VALUE : num(value.remnoise, 0)}
        reading={link}
      />
      <Readout
        label="Buffer free"
        note="back-pressure at 0"
        value={value === null ? NO_VALUE : num(value.txbufPct, 0)}
        unit="%"
        reading={link}
      />
      <Readout
        label="Receive errors"
        note="cumulative"
        value={value === null ? NO_VALUE : num(value.rxerrors, 0)}
        reading={link}
      />
    </>
  );
}
