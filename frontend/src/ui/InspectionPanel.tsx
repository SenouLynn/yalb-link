/**
 * The consulted tier: guidance, home, and radio link.
 *
 * These three families are read before and after a flight rather than during
 * one, so they live in the rail beside the other key→value groups rather than
 * competing for height with the instruments. They are rows, not readouts: each
 * family is one source, so provenance is stated once on the group header.
 *
 * A family that has never been heard collapses to a sentence rather than to a
 * column of dashes. The two conditions are genuinely different: a dash means a
 * value stopped arriving and the operator should notice, while RADIO_STATUS on
 * a UDP link is not late — it is not part of this system. Spending six dashes
 * on that would teach the operator to read dashes as decoration.
 *
 * Stale is not absent: a value inside a group that has gone stale still renders
 * as `- - -` under a draining header, which is the posture every other
 * instrument on the display already takes.
 */

import { bearing, num, signed, NO_VALUE } from './format';
import { Group, Row } from './primitives';
import { hasDisplayValue, type FlightReadings, type Reading } from './readings';

/** Tone for a value that is present or withheld. */
function tone(value: unknown): 'normal' | 'dead' {
  return value === null ? 'dead' : 'normal';
}

/** Absent when the family has never arrived at all. */
function absent(reading: Reading<unknown>, sentence: string): string | undefined {
  return reading.state === 'unavailable' ? sentence : undefined;
}

export function GuidancePanel({ readings }: { readings: FlightReadings }) {
  const { guidance } = readings;
  const value = hasDisplayValue(guidance) ? guidance.value : null;

  return (
    <Group
      label="Guidance"
      reading={guidance}
      absent={absent(
        guidance,
        'Not received. The autopilot reports it while it is navigating.',
      )}
    >
      <Row
        label="Target bearing"
        note="to waypoint"
        value={value === null ? NO_VALUE : bearing(value.targetBearingDeg)}
        unit="°"
        tone={tone(value)}
      />
      <Row
        label="Nav bearing"
        note="commanded"
        value={value === null ? NO_VALUE : bearing(value.navBearingDeg)}
        unit="°"
        tone={tone(value)}
      />
      <Row
        label="Waypoint distance"
        value={value === null ? NO_VALUE : num(value.wpDistM, 0)}
        unit="m"
        tone={tone(value)}
      />
      <Row
        label="Crosstrack"
        note="+ right of leg"
        value={value === null ? NO_VALUE : signed(value.xtrackErrorM)}
        unit="m"
        tone={tone(value)}
      />
      <Row
        label="Altitude error"
        note="+ below target"
        value={value === null ? NO_VALUE : signed(value.altErrorM)}
        unit="m"
        tone={tone(value)}
      />
      <Row
        label="Airspeed error"
        note="+ below target"
        value={value === null ? NO_VALUE : signed(value.aspdErrorMps)}
        unit="m/s"
        tone={tone(value)}
      />
    </Group>
  );
}

export function HomePanel({ readings }: { readings: FlightReadings }) {
  const { home } = readings;
  const value = hasDisplayValue(home) ? home.value : null;

  return (
    <Group
      label="Home"
      reading={home}
      absent={absent(
        home,
        'Not received. ArduPilot sends home when home is set, not on an interval.',
      )}
    >
      <Row
        label="Home position"
        value={
          value === null ? NO_VALUE : `${value.latDeg.toFixed(6)}, ${value.lonDeg.toFixed(6)}`
        }
        tone={tone(value)}
        stacked
      />
      <Row
        label="Home elevation"
        note="above sea level"
        value={value === null ? NO_VALUE : num(value.altMslM)}
        unit="m"
        tone={tone(value)}
      />
    </Group>
  );
}

export function RadioLinkPanel({ readings }: { readings: FlightReadings }) {
  const { link } = readings;
  const value = hasDisplayValue(link) ? link.value : null;

  return (
    <Group
      label="Radio link"
      reading={link}
      absent={absent(link, 'Not received. Radio telemetry may be unavailable on this link.')}
    >
      <Row
        label="Signal"
        note="local"
        value={value === null ? NO_VALUE : num(value.rssi, 0)}
        tone={tone(value)}
      />
      <Row
        label="Signal"
        note="remote"
        value={value === null ? NO_VALUE : num(value.remrssi, 0)}
        tone={tone(value)}
      />
      <Row
        label="Noise"
        note="local"
        value={value === null ? NO_VALUE : num(value.noise, 0)}
        tone={tone(value)}
      />
      <Row
        label="Noise"
        note="remote"
        value={value === null ? NO_VALUE : num(value.remnoise, 0)}
        tone={tone(value)}
      />
      <Row
        label="Buffer free"
        note="back-pressure at 0"
        value={value === null ? NO_VALUE : num(value.txbufPct, 0)}
        unit="%"
        tone={tone(value)}
      />
      <Row
        label="Receive errors"
        note="cumulative"
        value={value === null ? NO_VALUE : num(value.rxerrors, 0)}
        tone={tone(value)}
      />
    </Group>
  );
}

/**
 * All three consulted groups together.
 *
 * The rail registers them individually so the operator can hide one without
 * the others, but they are one tier and are tested and previewed as one.
 */
export function InspectionPanel({ readings }: { readings: FlightReadings }) {
  return (
    <>
      <GuidancePanel readings={readings} />
      <HomePanel readings={readings} />
      <RadioLinkPanel readings={readings} />
    </>
  );
}
