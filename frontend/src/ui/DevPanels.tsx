/**
 * The developer tier: what the decoder actually holds, before the display
 * decides what an operator may be shown.
 *
 * This is deliberately resident rather than summoned. The readings above it are
 * gated — a value that went stale four seconds ago is withheld and replaced by
 * a dash — and when an instrument shows a dash, the question is always whether
 * the aircraft stopped sending or the browser stopped decoding. These two
 * panels are the difference, and hunting for a debug flag to answer it is how
 * a display gets distrusted.
 *
 * Nothing here is operator-facing. Raw enum values and wire field names belong
 * to whoever is writing the decoder.
 */

import { TELEMETRY_TTL_MS, type VehicleView } from '@/fleet/state';

import { age, num } from './format';
import { Group, Row } from './primitives';

/**
 * Every MAVLink family the backend has observed, with the age of its last
 * message.
 *
 * `familySeenMs` is the backend's `observed_at`, so this is what the display
 * itself ages values against — the same clock the provenance drains use, shown
 * without the gating.
 */
export function FamiliesPanel({ view, nowMs }: { view: VehicleView; nowMs: number }) {
  const families = Object.entries(view.familySeenMs).sort(([a], [b]) => a.localeCompare(b));

  if (families.length === 0) {
    return (
      <Group
        label="Families"
        absent="Nothing decoded for this vehicle yet. The backend reports a family the first time it arrives."
      />
    );
  }

  const fresh = families.filter(([, seenMs]) => nowMs - seenMs <= TELEMETRY_TTL_MS).length;

  return (
    <Group label="Families" annotation={`${String(fresh)}/${String(families.length)} fresh`}>
      {families.map(([family, seenMs]) => {
        const ageMs = Math.max(0, nowMs - seenMs);

        return (
          <Row
            key={family}
            label={family}
            value={age(ageMs)}
            tone={ageMs > TELEMETRY_TTL_MS ? 'caution' : 'normal'}
          />
        );
      })}
    </Group>
  );
}

/**
 * The merged telemetry sample, field by field.
 *
 * Latest value per field wins and merged fields never expire on their own, so a
 * field can sit here indefinitely holding something that stopped being true —
 * which is exactly what the Families panel beside it is for.
 */
export function SamplePanel({ view }: { view: VehicleView }) {
  const fields = Object.entries(view.sample as unknown as Record<string, unknown>)
    .filter(([, value]) => value !== undefined && value !== null)
    .sort(([a], [b]) => a.localeCompare(b));

  if (fields.length === 0) {
    return <Group label="Raw sample" absent="No telemetry fields merged for this vehicle yet." />;
  }

  return (
    <Group label="Raw sample" annotation={`${String(fields.length)} fields`}>
      {fields.map(([field, value]) => (
        <Row key={field} label={field} value={formatWire(value)} stacked={isLong(value)} />
      ))}
    </Group>
  );
}

/**
 * Wire values, printed as they are.
 *
 * No units and no rounding to a display precision: the point of this panel is
 * to show what arrived, and a value cleaned up on its way to the screen cannot
 * settle an argument about what the decoder produced.
 */
function formatWire(value: unknown): string {
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : num(value, 4);
  }

  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }

  if (typeof value === 'bigint') {
    return String(value);
  }

  if (Array.isArray(value)) {
    return `[${value.map((entry) => formatWire(entry)).join(', ')}]`;
  }

  if (typeof value === 'object') {
    return JSON.stringify(value);
  }

  if (typeof value === 'string') {
    return value;
  }

  // Symbols and functions do not arrive over the wire, but a decoder change
  // could put one here, and a panel that exists to show the truth must not
  // silently print "[object Object]".
  return typeof value;
}

function isLong(value: unknown): boolean {
  return formatWire(value).length > 18;
}
