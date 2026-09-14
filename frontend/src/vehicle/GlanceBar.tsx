/**
 * The at-a-glance strip: what an operator wants true at every moment.
 *
 * Lives in the app bar rather than the view bar underneath it — the one row
 * that never scrolls out of reach regardless of which view or panel is on
 * screen — because these are exactly the facts a panel toggle must never be
 * able to take away. Ordered by what ends a sortie soonest: power, then link,
 * then position, then endurance.
 *
 * Three of these render as a coloured status icon rather than a labelled
 * number: battery, link and GPS. That is a deliberate, narrow exception to
 * this app's "no good green" rule — see `--status-good` in `system.css` for
 * why a status icon earns a healthy colour where a data row never does. The
 * icon's colour is the headline; the exact number is a beat behind it, on the
 * pointer and, for battery, spelled out beside the icon.
 */

import { GpsFixType } from '@/gen/gcs/v1/types_pb';
import type { BatteryResult } from '@/logic/battery';
import type { LinkQualityResult } from '@/logic/link';
import { isFamilyFresh } from '@/fleet/state';
import type { VehicleView } from '@/fleet/state';

import { NO_VALUE, age, num } from '@/ui/format';
import { Glance, type GlanceProps } from '@/ui/primitives';
import { hasDisplayValue, type FlightReadings, type Reading } from '@/ui/readings';
import { fixName, gpsFixOf } from '@/ui/StatusBar';

/** The four states a status icon can show. Independent of `Tone`: a data row
 *  never gets a "things are fine" colour, but a status icon whose entire job
 *  is answering "is this okay?" needs one, or it is not answering anything. */
export type StatusTone = 'good' | 'caution' | 'critical' | 'dead';

/** Distance in the unit that keeps it three or four characters wide. */
export function distance(metres: number | null): { value: string; unit: string } {
  if (metres === null || !Number.isFinite(metres)) {
    return { value: NO_VALUE, unit: '' };
  }

  return metres < 1000
    ? { value: metres.toFixed(0), unit: 'm' }
    : { value: (metres / 1000).toFixed(2), unit: 'km' };
}

/** Elapsed flight time as `m:ss`, or `h:mm:ss` once it runs past the hour. */
export function elapsed(ms: number | null): string {
  if (ms === null || !Number.isFinite(ms) || ms < 0) {
    return NO_VALUE;
  }

  const total = Math.floor(ms / 1000);
  const seconds = String(total % 60).padStart(2, '0');
  const minutes = Math.floor(total / 60) % 60;
  const hours = Math.floor(total / 3600);

  return hours === 0
    ? `${String(minutes)}:${seconds}`
    : `${String(hours)}:${String(minutes).padStart(2, '0')}:${seconds}`;
}

/** Source and age for the pointer, since the strip draws no provenance meter. */
function provenance(reading: Reading<unknown>): string {
  if (reading.source === null) {
    return 'Never received';
  }

  return `${reading.source} · ${age(reading.ageMs)} · ${reading.state}`;
}

/** Whether the family behind a reading has ever arrived at all. */
function reported(reading: Reading<unknown>): boolean {
  return reading.state !== 'unavailable';
}

/** Plain English for a tone, so the tooltip states the icon's colour in
 *  words rather than leaving an operator to infer it. */
function toneWord(tone: StatusTone): string {
  switch (tone) {
    case 'good': return 'healthy';
    case 'caution': return 'caution';
    case 'critical': return 'critical';
    case 'dead': return 'no data';
  }
}

/**
 * A cell voltage graded against a fixed threshold, not a percentage.
 *
 * State of charge is a coulomb-counting estimate that drifts; a LiPo's own
 * voltage sag near empty is the thing that actually predicts how much flight
 * is left, which is why the operator asked for this to key off volts per cell
 * rather than the remaining-percent BATTERY_STATUS already reports. It grades
 * only when the pack reported itself as individual cells — a `SYS_STATUS`-only
 * pack voltage carries no cell count, and dividing by a guessed one would be
 * fabricated precision on the one reading an operator uses to decide whether
 * to land.
 */
export function batteryTone(battery: BatteryResult | null): StatusTone {
  if (battery?.voltageV == null || battery.cellCount === null) {
    return 'dead';
  }

  const perCell = battery.voltageV / battery.cellCount;

  if (perCell <= 3.4) return 'critical';
  if (perCell < 3.8) return 'caution';

  return 'good';
}

/**
 * The weaker direction decides the icon: a link is only as good as its worst
 * leg. Thresholds are this display's own judgement call, not a MAVLink or SiK
 * spec — RADIO_STATUS rssi has no defined "good" value, only a 0..254 scale.
 */
export function linkTone(link: LinkQualityResult | null): StatusTone {
  if (link === null) return 'dead';

  const values = [link.rssi, link.remrssi].filter((value): value is number => value !== null);

  if (values.length === 0) return 'dead';

  const weakest = Math.min(...values);

  if (weakest >= 180) return 'good';
  if (weakest >= 80) return 'caution';

  return 'critical';
}

/**
 * GPS as four states rather than `StatusBar`'s two: a 2D fix is flyable but
 * degraded, and collapsing it into the same "fine" bucket as a 3D fix would
 * hide exactly the difference an operator glances at this icon to catch.
 */
export function gpsTone(view: VehicleView, nowMs: number): StatusTone {
  const fix = gpsFixOf(view);

  if (fix === undefined || !isFamilyFresh(view, 'GPS_RAW_INT', nowMs)) {
    return 'dead';
  }

  if (fix >= GpsFixType.GPS_FIX_TYPE_3D_FIX) return 'good';
  if (fix === GpsFixType.GPS_FIX_TYPE_2D_FIX) return 'caution';

  return 'critical';
}

/** GPS_RAW_INT's own age. `readings.position` is the wrong family here — it
 *  resolves from GLOBAL_POSITION_INT, a fix estimate the EKF can still report
 *  after the raw GPS has gone stale, and the icon above it is graded on the
 *  raw fix, not the estimate. */
function gpsAgeMs(view: VehicleView, nowMs: number): number | null {
  const seen = view.familySeenMs['GPS_RAW_INT'];
  return seen === undefined ? null : Math.max(0, nowMs - seen);
}

/** A battery outline with a charge-level fill, coloured by `batteryTone`. */
function BatteryIcon({ tone, pct }: { tone: StatusTone; pct: number | null }) {
  const fraction = pct === null ? 0 : Math.max(0, Math.min(100, pct)) / 100;
  // design-geometry: a battery body plus its terminal nub, drawn to a fixed
  // 18x10 box rather than derived from a token — it is an instrument face,
  // like the attitude ladder, not a spacing value.
  const fillWidth = 12 * fraction;

  return (
    <svg
      className={`status-icon status-icon--${tone}`}
      width="18" height="10" viewBox="0 0 18 10" role="img" aria-hidden="true"
    >
      <rect x="0.5" y="0.5" width="15" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1" />
      <rect x="16" y="3" width="1.5" height="4" fill="currentColor" />
      {fillWidth > 0 && <rect x="2" y="2" width={fillWidth} height="6" fill="currentColor" />}
    </svg>
  );
}

/** Three ascending bars, like a phone's signal glyph. Lit bar count carries
 *  the tone a second way, for an operator who cannot resolve the colour. */
function SignalIcon({ tone }: { tone: StatusTone }) {
  const lit = tone === 'good' ? 3 : tone === 'caution' ? 2 : tone === 'critical' ? 1 : 0;
  const bars = [
    { x: 0, h: 4 },
    { x: 5, h: 7 },
    { x: 10, h: 10 },
  ];

  return (
    <svg
      className={`status-icon status-icon--${tone}`}
      width="14" height="10" viewBox="0 0 14 10" role="img" aria-hidden="true"
    >
      {bars.map((bar, index) => (
        <rect
          key={bar.x}
          x={bar.x} y={10 - bar.h} width="3.5" height={bar.h}
          fill={index < lit ? 'currentColor' : 'none'}
          stroke="currentColor" strokeWidth="1"
        />
      ))}
    </svg>
  );
}

/** A satellite: a body, two panel wings, and an antenna — coloured by
 *  `gpsTone`. Replaces an earlier plain dot, which read as "a location" and
 *  not as "the constellation this fix comes from". */
function GpsIcon({ tone }: { tone: StatusTone }) {
  return (
    <svg
      className={`status-icon status-icon--${tone}`}
      width="14" height="12" viewBox="0 0 14 12" role="img" aria-hidden="true"
    >
      <rect x="0.5" y="4.5" width="3" height="3" fill="none" stroke="currentColor" strokeWidth="1" />
      <rect x="10.5" y="4.5" width="3" height="3" fill="none" stroke="currentColor" strokeWidth="1" />
      <line x1="3.5" y1="6" x2="5" y2="6" stroke="currentColor" strokeWidth="1" />
      <line x1="9" y1="6" x2="10.5" y2="6" stroke="currentColor" strokeWidth="1" />
      <rect x="5" y="4.5" width="4" height="3" fill="currentColor" transform="rotate(45 7 6)" />
      <line x1="8.5" y1="4" x2="10" y2="2" stroke="currentColor" strokeWidth="1" />
      <circle cx="10.3" cy="1.7" r="0.6" fill="currentColor" />
    </svg>
  );
}

export function GlanceBar({
  view,
  readings,
  nowMs,
}: {
  view: VehicleView;
  readings: FlightReadings;
  nowMs: number;
}) {
  const battery = hasDisplayValue(readings.battery) ? readings.battery.value : null;
  const link = hasDisplayValue(readings.link) ? readings.link.value : null;
  const power = provenance(readings.battery);
  const radio = provenance(readings.link);

  const batteryVoltageV = battery?.voltageV ?? null;
  const batteryRemainingPct = battery?.remainingPct ?? null;
  const batteryValue = [
    batteryVoltageV === null ? null : `${num(batteryVoltageV, 1)}V`,
    batteryRemainingPct === null ? null : `/${num(batteryRemainingPct, 0)}%`,
  ].filter((part): part is string => part !== null).join(' ');

  const gpsFix = gpsFixOf(view);
  const gpsFresh = isFamilyFresh(view, 'GPS_RAW_INT', nowMs);
  const sats = view.sample.satellitesVisible;
  const fixText = gpsFresh && gpsFix !== undefined ? fixName(gpsFix) : NO_VALUE;
  const satsText = gpsFresh && sats !== undefined ? String(sats) : NO_VALUE;

  // Flown, not tracked: the odometer is accumulated in the fold precisely so
  // this number cannot shrink when the 500-point track ring starts evicting.
  const flying = view.firstFixAtMs !== undefined;
  const flown = distance(flying ? view.odometerM : null);

  const batteryStatusTone = batteryTone(battery);
  const linkStatusTone = linkTone(link);
  const gpsStatusTone = gpsTone(view, nowMs);
  const perCellV = battery?.voltageV != null && battery.cellCount !== null
    ? battery.voltageV / battery.cellCount : null;

  const batteryTitle = perCellV === null
    ? `${power} — no per-cell reading, cannot grade`
    : `${num(perCellV, 2)}V/cell (${toneWord(batteryStatusTone)}) · ${power}`;

  const linkValue = link === null ? NO_VALUE : `${num(link.rssi, 0)} · ${num(link.remrssi, 0)}`;
  const linkTitle = `Local ${num(link?.rssi, 0)} · Remote ${num(link?.remrssi, 0)} (${toneWord(linkStatusTone)}) · ${radio}`;

  const gpsTitle = gpsFresh
    ? `${toneWord(gpsStatusTone)} · GPS_RAW_INT · ${age(gpsAgeMs(view, nowMs))}`
    : 'No data — GPS_RAW_INT stale or never received';

  return (
    <>
      {reported(readings.battery) && (
        <span className="status-glance" title={batteryTitle}>
          <BatteryIcon tone={batteryStatusTone} pct={batteryRemainingPct} />
          <span className="status-glance__value">{batteryValue === '' ? NO_VALUE : batteryValue}</span>
        </span>
      )}

      {/* Local · remote, the same pairing the tone itself grades on (the
          weaker of the two) — not just the icon's colour to go on. */}
      {reported(readings.link) && (
        <span className="status-glance" title={linkTitle}>
          <SignalIcon tone={linkStatusTone} />
          <span className="status-glance__value">{linkValue}</span>
        </span>
      )}

      {/* Fix type reads as plain text ("3D") before the icon; the icon sits
          against the number it actually labels — the satellite count, not
          the fix type beside it — the way an iNav OSD pairs a satellite
          glyph with its count rather than with the fix-quality text. */}
      <span className="status-glance" title={gpsTitle}>
        <span className="status-glance__value">{fixText}</span>
        <GpsIcon tone={gpsStatusTone} />
        <span className="status-glance__value">{satsText}</span>
      </span>

      {items(readings, view, nowMs, flying, flown).map((item) => (
        <Glance key={item.label} {...item} />
      ))}
    </>
  );
}

function items(
  readings: FlightReadings,
  view: VehicleView,
  nowMs: number,
  flying: boolean,
  flown: { value: string; unit: string },
): GlanceProps[] {
  const endurance: GlanceProps[] = flying ? [
    {
      label: 'Flown',
      value: flown.value,
      unit: flown.unit,
      tone: 'normal',
      title: 'Great-circle distance accumulated over every position fix received',
    },
    {
      label: 'Elapsed',
      value: elapsed(view.firstFixAtMs === undefined ? null : nowMs - view.firstFixAtMs),
      tone: 'normal',
      title: 'Since the first position fix, not since the first heartbeat',
    },
  ] : [];

  const altitude = hasDisplayValue(readings.position) ? readings.position.value : null;
  const air = hasDisplayValue(readings.airspeed) ? readings.airspeed.value : null;

  const flightData: (GlanceProps | false)[] = [
    reported(readings.position) && {
      label: 'Altitude',
      value: altitude === null ? NO_VALUE : num(altitude.altM, 0),
      unit: 'm',
      tone: altitude === null ? 'dead' : 'normal',
      title: 'Above home · GLOBAL_POSITION_INT',
    },
    reported(readings.airspeed) && {
      label: 'Airspeed',
      value: air === null ? NO_VALUE : num(air.airspeedMps, 1),
      unit: 'm/s',
      tone: air === null ? 'dead' : 'normal',
      title: 'VFR_HUD',
    },
  ];

  return [...endurance, ...flightData.filter((item): item is GlanceProps => item !== false)];
}

