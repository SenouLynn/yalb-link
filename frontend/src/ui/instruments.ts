/**
 * Instrument geometry.
 *
 * Kept as pure functions so the arithmetic that decides where a horizon line
 * lands can be tested directly, rather than inferred from rendered markup.
 */

/**
 * Degrees of pitch shown from the centre of the horizon to its edge.
 *
 * Narrow on purpose. A multirotor's useful pitch range is small, and a scale
 * wide enough to show ±90° would make ordinary flight look motionless.
 */
export const PITCH_RANGE_DEG = 30;

/** Degrees of heading visible across the compass tape. */
export const HEADING_SPAN_DEG = 90;

export interface HorizonTransform {
  /** Vertical offset of the horizon, in viewBox units. Positive is down. */
  translateY: number;
  /** Rotation applied to the horizon, in degrees. Opposes the roll. */
  rotateDeg: number;
}

/**
 * Places the horizon for a given attitude.
 *
 * The horizon moves opposite the aircraft: rolling right tips the visible
 * horizon left. Getting this backwards produces an instrument that is
 * confidently, dangerously wrong, so it is stated once here and tested.
 */
export function horizonTransform(
  rollDeg: number,
  pitchDeg: number,
  halfHeight: number,
): HorizonTransform {
  const clamped = Math.max(-PITCH_RANGE_DEG, Math.min(PITCH_RANGE_DEG, pitchDeg));

  return {
    translateY: (clamped / PITCH_RANGE_DEG) * halfHeight,
    rotateDeg: -rollDeg,
  };
}

export interface HeadingTick {
  /** Bearing this tick marks. */
  deg: number;
  /** Horizontal position across the tape, 0..1 from left edge to right. */
  offset: number;
  /** Ticks on a multiple of 30° are labelled; the rest are hash marks. */
  major: boolean;
}

/**
 * Builds the visible ticks of the compass tape.
 *
 * Emits absolute bearings with a fractional position, so the caller renders
 * the tape without doing any wrapping arithmetic of its own — which is where a
 * compass that jumps at north comes from.
 */
export function headingTicks(
  headingDeg: number,
  spanDeg: number = HEADING_SPAN_DEG,
  stepDeg = 10,
): HeadingTick[] {
  const half = spanDeg / 2;
  const first = Math.ceil((headingDeg - half) / stepDeg) * stepDeg;

  const ticks: HeadingTick[] = [];

  for (let deg = first; deg <= headingDeg + half; deg += stepDeg) {
    ticks.push({
      deg: ((deg % 360) + 360) % 360,
      offset: (deg - (headingDeg - half)) / spanDeg,
      major: (((deg % 360) + 360) % 360) % 30 === 0,
    });
  }

  return ticks;
}

/** Point on a circle, for the compass rose. Zero degrees is up. */
export function polar(cx: number, cy: number, radius: number, deg: number): [number, number] {
  const rad = ((deg - 90) * Math.PI) / 180;

  return [cx + radius * Math.cos(rad), cy + radius * Math.sin(rad)];
}
