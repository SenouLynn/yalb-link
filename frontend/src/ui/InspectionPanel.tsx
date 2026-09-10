/**
 * The consulted families: guidance, home, and radio link.
 *
 * These are read before and after a flight rather than during one, so they live
 * beside the other key→value groups rather than competing for height with the
 * instruments. They are rows, not readouts: each family is one source, so
 * provenance is stated once on the group header — except home, which has no
 * group of its own any more and says so on its row.
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

import { age, bearing, latLon, num, signed, NO_VALUE } from './format';
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
        hint="to waypoint"
        value={value === null ? NO_VALUE : bearing(value.targetBearingDeg)}
        unit="°"
        tone={tone(value)}
      />
      <Row
        label="Nav bearing"
        hint="commanded"
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
        hint="positive is right of the leg"
        value={value === null ? NO_VALUE : signed(value.xtrackErrorM)}
        unit="m"
        tone={tone(value)}
      />
      <Row
        label="Altitude error"
        hint="positive is below target"
        value={value === null ? NO_VALUE : signed(value.altErrorM)}
        unit="m"
        tone={tone(value)}
      />
      <Row
        label="Airspeed error"
        hint="positive is below target"
        value={value === null ? NO_VALUE : signed(value.aspdErrorMps)}
        unit="m/s"
        tone={tone(value)}
      />
    </Group>
  );
}

/**
 * Home, as rows inside the panel that carries the position they qualify.
 *
 * No header of its own. Home is the datum the position above it and the
 * altitude tape across the screen are both measured against, and a section
 * heading between them made it read as an unrelated fact that happened to be
 * nearby.
 *
 * The source is named on the first row's note instead. That is not the per-row
 * provenance the Once-Per-Group Rule forbids — what that rule is about is a
 * second right-aligned value and a second drain track competing with the value
 * edge. This is a left-hand line in the slot meant for what the operator cannot
 * infer, and it is needed here precisely because these two rows do NOT come
 * from the family the group header names: HOME_POSITION arrives when home is
 * set and never again, so the header's five-second drain would otherwise be
 * claiming a freshness these rows do not have.
 */
export function HomeRows({ readings }: { readings: FlightReadings }) {
  const { home } = readings;
  const value = hasDisplayValue(home) ? home.value : null;

  // Never received is a different fact from "stopped arriving", and on a datum
  // it is the one that changes what the operator does: an unset home is why the
  // altitude beside it cannot be trusted as height above the launch point.
  if (home.state === 'unavailable') {
    return (
      <Row
        label="Home"
        hint="set by the vehicle, not sent on an interval"
        value="NOT SET"
        tone="dead"
      />
    );
  }

  return (
    <>
      <Row
        label="Home"
        hint={`${home.source ?? 'HOME_POSITION'} · ${age(home.ageMs)}`}
        value={latLon(value?.latDeg, value?.lonDeg)}
        tone={tone(value)}
      />
      <Row
        label="Home elevation"
        hint="above sea level"
        value={value === null ? NO_VALUE : num(value.altMslM)}
        unit="m"
        tone={tone(value)}
      />
    </>
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
      {/* Local and remote belong in the label, not on the pointer: they are
          the only thing telling two identically named rows apart, and a
          difference the operator has to hover to find is not a difference. */}
      <Row
        label="Signal · local"
        value={value === null ? NO_VALUE : num(value.rssi, 0)}
        tone={tone(value)}
      />
      <Row
        label="Signal · remote"
        value={value === null ? NO_VALUE : num(value.remrssi, 0)}
        tone={tone(value)}
      />
      <Row
        label="Noise · local"
        value={value === null ? NO_VALUE : num(value.noise, 0)}
        tone={tone(value)}
      />
      <Row
        label="Noise · remote"
        value={value === null ? NO_VALUE : num(value.remnoise, 0)}
        tone={tone(value)}
      />
      <Row
        label="Buffer free"
        hint="back-pressure at 0"
        value={value === null ? NO_VALUE : num(value.txbufPct, 0)}
        unit="%"
        tone={tone(value)}
      />
      <Row
        label="Receive errors"
        hint="cumulative"
        value={value === null ? NO_VALUE : num(value.rxerrors, 0)}
        tone={tone(value)}
      />
    </Group>
  );
}

/**
 * All three together.
 *
 * The registry mounts them separately so the operator can hide one without the
 * others — and home is no longer separate at all, it is rows inside Position —
 * but they are one tier and are previewed as one.
 */
export function InspectionPanel({ readings }: { readings: FlightReadings }) {
  return (
    <>
      <GuidancePanel readings={readings} />
      <Group label="Position"><HomeRows readings={readings} /></Group>
      <RadioLinkPanel readings={readings} />
    </>
  );
}
