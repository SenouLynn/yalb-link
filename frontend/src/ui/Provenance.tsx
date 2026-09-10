/** The line under every reading: where it came from, and how old it is. */

import { age, NO_VALUE } from './format';
import { stalenessFraction, type Reading } from './readings';

export interface ProvenanceProps {
  reading: Reading<unknown>;
}

/**
 * Shows the MAVLink family a value resolved from and its age, over a rule that
 * drains as the value approaches its TTL.
 *
 * The bar carries the same information as the age text, on purpose. Reading a
 * number takes attention the operator is spending on the aircraft; a bar that
 * is visibly emptying does not.
 */
export function Provenance({ reading }: ProvenanceProps) {
  const remaining = 1 - stalenessFraction(reading);

  const modifier =
    reading.state === 'unavailable'
      ? 'provenance--unavailable'
      : reading.state === 'stale'
        ? 'provenance--stale'
        : '';

  return (
    <div className={`provenance ${modifier}`.trim()}>
      <div className="provenance__track">
        <div
          className="provenance__fill"
          style={{ transform: `scaleX(${remaining.toFixed(3)})` }}
        />
      </div>
      <div className="provenance__text">
        <span>{reading.source ?? 'NO SOURCE'}</span>
        <span>{reading.state === 'unavailable' ? NO_VALUE : age(reading.ageMs)}</span>
      </div>
    </div>
  );
}
