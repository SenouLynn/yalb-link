/** One labelled number with its provenance. */

import { NO_VALUE } from './format';
import { Provenance } from './Provenance';
import type { Reading } from './readings';

export interface ReadoutProps {
  label: string;
  /** Pre-formatted value. Pass NO_VALUE when there is nothing to show. */
  value: string;
  unit?: string | undefined;
  /** Extra context under the label, such as the altitude datum. */
  note?: string | undefined;
  reading: Reading<unknown>;
  wide?: boolean | undefined;
}

/**
 * A readout renders whatever string it is given, but takes the reading so it
 * can colour and annotate it consistently. Missing and stale values are both
 * withheld; provenance distinguishes which condition caused the dash.
 */
export function Readout({ label, value, unit, note, reading, wide }: ReadoutProps) {
  const unavailable = value === NO_VALUE;

  const valueClass = unavailable ? 'value value--unavailable' : 'value';

  return (
    <div className={`panel ${wide === true ? 'readout--wide' : ''}`.trim()}>
      <div className="label">
        {label}
        {note !== undefined && note !== '' ? ` · ${note}` : ''}
      </div>
      <div className={valueClass}>
        {value}
        {unit !== undefined && unit !== '' ? (
          <span className={unavailable ? 'unit unit--unavailable' : 'unit'}>{unit}</span>
        ) : null}
      </div>
      <Provenance reading={reading} />
    </div>
  );
}
