/** One labelled number with its provenance. */

import { NO_VALUE } from './format';
import { Provenance } from './Provenance';
import { Row } from './primitives';
import type { Reading } from './readings';

export interface ReadoutProps {
  label: string;
  /** Pre-formatted value. Pass NO_VALUE when there is nothing to show. */
  value: string;
  unit?: string | undefined;
  /** Extra context under the label, such as the altitude datum. */
  note?: string | undefined;
  reading: Reading<unknown>;
  /** The one available emphasis: a group's headline value. Use once per group. */
  lead?: boolean | undefined;
}

/**
 * A readout is a row that carries its own provenance.
 *
 * It renders whatever string it is given, but takes the reading so it can
 * colour and annotate it consistently. Missing and stale values are both
 * withheld; provenance distinguishes which condition caused the dash.
 *
 * It is deliberately a row and not a card. Six readings in a column have to
 * scan as one table against a shared value edge — as bordered cards with large
 * numbers they read as six separate objects, which is both slower to scan and
 * three times the screen area for the same information.
 */
export function Readout({ label, value, unit, note, reading, lead }: ReadoutProps) {
  const unavailable = value === NO_VALUE;

  return (
    <div className="reading">
      <Row
        label={note === undefined || note === '' ? label : `${label} · ${note}`}
        value={value}
        unit={unit}
        tone={unavailable ? 'dead' : 'normal'}
        lead={lead}
      />
      <Provenance reading={reading} />
    </div>
  );
}
