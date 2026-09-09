/** Compass tape, with the bearing read against a fixed lubber line. */

import type { HeadingResult } from '@/logic/heading';

import { bearing, cardinal, NO_VALUE } from './format';
import { headingTicks, HEADING_SPAN_DEG } from './instruments';
import { Provenance } from './Provenance';
import { hasDisplayValue, type Reading } from './readings';

const WIDTH = 240;
const HEIGHT = 64;
const BASELINE = 44;
const VIEW_BOX = `0 0 ${String(WIDTH)} ${String(HEIGHT)}`;

export interface HeadingIndicatorProps {
  reading: Reading<HeadingResult>;
}

/**
 * Draws the heading tape.
 *
 * With no heading the tape is blank rather than parked at north, for the same
 * reason the horizon does not sit level: 000 is a bearing, not an absence.
 */
export function HeadingIndicator({ reading }: HeadingIndicatorProps) {
  const heading = hasDisplayValue(reading) ? reading.value : null;
  const ticks = heading === null ? [] : headingTicks(heading.headingDeg);

  return (
    <div className="panel instrument instrument--heading">
      <div className="label">
        Heading
        {heading?.isFallback === true ? ' · fallback source' : ''}
      </div>

      <svg
        viewBox={VIEW_BOX}
        role="img"
        aria-label={
          heading === null ? 'Heading unavailable' : `Heading ${bearing(heading.headingDeg)}`
        }
      >
        <rect x="0" y="0" width={WIDTH} height={HEIGHT} rx="2" fill="var(--panel-deep)" />

        {ticks.map((tick) => {
          const x = tick.offset * WIDTH;
          const point = cardinal(tick.deg);

          return (
            <g key={`${String(tick.deg)}-${tick.offset.toFixed(3)}`}>
              <line
                x1={x}
                y1={tick.major ? BASELINE - 14 : BASELINE - 7}
                x2={x}
                y2={BASELINE}
                stroke="var(--lume-dim)"
                strokeWidth="1"
              />
              {tick.major ? (
                <text
                  x={x}
                  y={BASELINE - 18}
                  textAnchor="middle"
                  fontSize="9"
                  fontFamily="var(--data)"
                  fill="var(--lume)"
                >
                  {point ?? tick.deg}
                </text>
              ) : null}
            </g>
          );
        })}

        {/* Lubber line: the tape moves, this does not. */}
        <path
          d={`M ${String(WIDTH / 2)} ${String(BASELINE + 8)} l -5 8 h 10 z`}
          fill="var(--lume)"
        />
        <line
          x1={WIDTH / 2}
          y1={BASELINE - 20}
          x2={WIDTH / 2}
          y2={BASELINE + 8}
          stroke="var(--lume)"
          strokeWidth="1.5"
        />

        <rect
          x="0.5"
          y="0.5"
          width={WIDTH - 1}
          height={HEIGHT - 1}
          rx="2"
          fill="none"
          stroke="var(--bezel)"
        />
      </svg>

      <div>
        <div className="label">Bearing · {String(HEADING_SPAN_DEG)}° shown</div>
        <div className={valueClass(reading)}>
          {heading === null ? NO_VALUE : bearing(heading.headingDeg)}
          <span className="unit">°</span>
        </div>
      </div>

      <Provenance reading={reading} />
    </div>
  );
}

function valueClass(reading: Reading<HeadingResult>): string {
  if (!hasDisplayValue(reading)) {
    return 'value value--unavailable';
  }

  return 'value';
}
