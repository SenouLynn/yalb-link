/** Artificial horizon. Blue over ochre, the way the hardware does it. */

import type { AttitudeResult } from '@/logic/attitude';

import { NO_VALUE, num } from './format';
import { horizonTransform, PITCH_RANGE_DEG } from './instruments';
import { Group, Row } from './primitives';
import { hasDisplayValue, type Reading } from './readings';

const WIDTH = 240;
const HEIGHT = 180;
const HALF = HEIGHT / 2;
const VIEW_BOX = `0 0 ${String(WIDTH)} ${String(HEIGHT)}`;

/** Pitch ladder rungs, in degrees either side of the horizon. */
const LADDER_DEG = [-20, -10, 10, 20];

export interface AttitudeIndicatorProps {
  reading: Reading<AttitudeResult>;
}

/**
 * Draws roll and pitch.
 *
 * When there is no attitude the instrument shows an explicitly empty face
 * rather than a level horizon. A level horizon is a claim — that the aircraft
 * is upright — and it is the single most misleading thing this display could
 * draw from missing data.
 */
export function AttitudeIndicator({ reading }: AttitudeIndicatorProps) {
  const attitude = hasDisplayValue(reading) ? reading.value : null;

  return (
    <Group label="Attitude" reading={reading} className="instrument">
      <svg viewBox={VIEW_BOX} role="img" aria-label={describe(attitude)}>
        <defs>
          <clipPath id="horizon-clip">
            <rect x="0" y="0" width={WIDTH} height={HEIGHT} rx="2" />
          </clipPath>
        </defs>

        <g clipPath="url(#horizon-clip)">
          {attitude === null ? (
            <rect x="0" y="0" width={WIDTH} height={HEIGHT} fill="var(--panel-deep)" />
          ) : (
            <Horizon rollDeg={attitude.rollDeg} pitchDeg={attitude.pitchDeg} />
          )}
        </g>

        {/* The aircraft reference stays fixed; the world moves behind it. */}
        <g stroke="var(--lume)" strokeWidth="2" fill="none">
          <path d={`M ${String(WIDTH / 2 - 46)} ${String(HALF)} h 28`} />
          <path d={`M ${String(WIDTH / 2 + 18)} ${String(HALF)} h 28`} />
          <circle cx={WIDTH / 2} cy={HALF} r="2.5" fill="var(--lume)" />
        </g>

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

      <Row
        label="Roll"
        value={attitude === null ? NO_VALUE : num(attitude.rollDeg)}
        unit="°"
        tone={hasDisplayValue(reading) ? 'normal' : 'dead'}
      />
      <Row
        label="Pitch"
        value={attitude === null ? NO_VALUE : num(attitude.pitchDeg)}
        unit="°"
        tone={hasDisplayValue(reading) ? 'normal' : 'dead'}
      />

    </Group>
  );
}

function Horizon({ rollDeg, pitchDeg }: { rollDeg: number; pitchDeg: number }) {
  const { translateY, rotateDeg } = horizonTransform(rollDeg, pitchDeg, HALF);

  // Oversized so the rotated rectangles still cover the corners.
  const span = WIDTH * 2;

  return (
    <g
      transform={`translate(${String(WIDTH / 2)} ${String(HALF)}) rotate(${String(rotateDeg)}) translate(0 ${String(translateY)})`}
    >
      <rect x={-span} y={-span} width={span * 2} height={span} fill="var(--sky)" />
      <rect x={-span} y={0} width={span * 2} height={span} fill="var(--ground)" />
      <line x1={-span} y1="0" x2={span} y2="0" stroke="var(--lume)" strokeWidth="1.5" />

      {LADDER_DEG.map((deg) => {
        const y = -(deg / PITCH_RANGE_DEG) * HALF;
        const halfWidth = Math.abs(deg) === 10 ? 18 : 28;

        return (
          <g key={deg} stroke="var(--lume)" strokeWidth="1" opacity="0.75">
            <line x1={-halfWidth} y1={y} x2={halfWidth} y2={y} />
          </g>
        );
      })}
    </g>
  );
}


function describe(attitude: AttitudeResult | null): string {
  if (attitude === null) {
    return 'Attitude unavailable';
  }

  return `Roll ${attitude.rollDeg.toFixed(0)} degrees, pitch ${attitude.pitchDeg.toFixed(0)} degrees`;
}
