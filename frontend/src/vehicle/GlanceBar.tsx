/**
 * The at-a-glance strip: what an operator wants true at every moment.
 *
 * These six readings are already on screen inside their own panels, and that is
 * the point — they are the ones whose absence for even a few seconds changes
 * what you do next, so they are also stated somewhere no panel toggle can take
 * them away. Endurance is the theme: how much power is left, whether the link
 * carrying the aircraft's voice is holding, and how far and how long it has
 * been flying.
 *
 * Ordered by what ends a sortie soonest: power, then link, then endurance.
 */

import { NO_VALUE, age, num } from '@/ui/format';
import { Glance, type GlanceProps } from '@/ui/primitives';
import { hasDisplayValue, type FlightReadings, type Reading } from '@/ui/readings';
import type { VehicleView } from '@/fleet/state';

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

  // Flown, not tracked: the odometer is accumulated in the fold precisely so
  // this number cannot shrink when the 500-point track ring starts evicting.
  const flying = view.firstFixAtMs !== undefined;
  const flown = distance(flying ? view.odometerM : null);

  /*
   * A family nobody has ever heard is omitted, not dashed.
   *
   * This is the rule the rail's consulted groups already follow, and the reason
   * is the same one: RADIO_STATUS comes from a SiK modem, so on a UDP link —
   * Compose SITL, and every bench session — it is not late, it is not part of
   * this system. Two permanent dashes on a bar the operator reads at a glance
   * is how they learn to read dashes as decoration, which is exactly what has
   * to keep working when a value that *was* arriving stops. Stale still dashes:
   * that one is news.
   */
  const items: (GlanceProps | null)[] = [
    reported(readings.battery) &&
      glance('Battery', num(battery?.voltageV, 1), 'V', battery?.voltageV, power),
    reported(readings.battery) &&
      glance('Remaining', num(battery?.remainingPct, 0), '%', battery?.remainingPct, power),
    reported(readings.link) &&
      glance('Signal', num(link?.rssi, 0), '', link?.rssi, `Local. ${radio}`),
    reported(readings.link) &&
      glance('Signal rem', num(link?.remrssi, 0), '', link?.remrssi, `Remote. ${radio}`),
    flying &&
      glance(
        'Flown',
        flown.value,
        flown.unit,
        view.odometerM,
        'Great-circle distance accumulated over every position fix received',
      ),
    flying &&
      glance(
        'Elapsed',
        elapsed(view.firstFixAtMs === undefined ? null : nowMs - view.firstFixAtMs),
        '',
        view.firstFixAtMs,
        'Since the first position fix, not since the first heartbeat',
      ),
  ].map((entry) => (entry === false ? null : entry));

  return (
    <>
      {items.map((item) =>
        item === null ? null : <Glance key={item.label} {...item} />,
      )}
    </>
  );
}

/** Whether the family behind a reading has ever arrived at all. */
function reported(reading: Reading<unknown>): boolean {
  return reading.state !== 'unavailable';
}

function glance(
  label: string,
  value: string,
  unit: string,
  present: number | null | undefined,
  title: string,
): GlanceProps {
  return {
    label,
    value,
    unit,
    // Absent, not disabled: a dash here is a value the operator has to read.
    tone: present === null || present === undefined ? 'dead' : 'normal',
    title,
  };
}
