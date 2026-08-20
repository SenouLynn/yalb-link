import { describe, expect, it } from 'vitest';

import { bearing, cardinal, NO_VALUE, num, signed, age } from './format';
import { headingTicks, horizonTransform, polar, PITCH_RANGE_DEG } from './instruments';

describe('horizonTransform', () => {
  it('leaves the horizon centred and level for a level aircraft', () => {
    const level = horizonTransform(0, 0, 100);

    // toBeCloseTo rather than toEqual: negating a zero roll yields -0, which
    // Object.is separates from 0 but which renders as "0" either way.
    expect(level.translateY).toBeCloseTo(0);
    expect(level.rotateDeg).toBeCloseTo(0);
  });

  it('rotates the horizon opposite the roll', () => {
    // Rolling right must tip the visible horizon left, not right. Getting this
    // backwards produces an instrument that is confidently wrong.
    expect(horizonTransform(30, 0, 100).rotateDeg).toBe(-30);
    expect(horizonTransform(-30, 0, 100).rotateDeg).toBe(30);
  });

  it('moves the horizon down as the nose pitches up', () => {
    expect(horizonTransform(0, 15, 100).translateY).toBeCloseTo(50);
    expect(horizonTransform(0, -15, 100).translateY).toBeCloseTo(-50);
  });

  it('clamps beyond the displayed pitch range rather than sliding off', () => {
    expect(horizonTransform(0, 90, 100).translateY).toBeCloseTo(100);
    expect(horizonTransform(0, -90, 100).translateY).toBeCloseTo(-100);
    expect(horizonTransform(0, PITCH_RANGE_DEG, 100).translateY).toBeCloseTo(100);
  });
});

describe('headingTicks', () => {
  it('spans the requested arc centred on the heading', () => {
    // Heading 90 over a 90-degree span covers 45..135; ticks land on the
    // multiples of the step inside it, so 50 through 130.
    const ticks = headingTicks(90, 90, 10);

    expect(ticks[0]?.deg).toBe(50);
    expect(ticks[ticks.length - 1]?.deg).toBe(130);
    expect(ticks.every((tick) => tick.offset >= 0 && tick.offset <= 1)).toBe(true);
  });

  it('wraps through north without a discontinuity', () => {
    const ticks = headingTicks(0, 90, 10);
    const degrees = ticks.map((tick) => tick.deg);

    // Both sides of north are present, expressed as absolute bearings.
    expect(degrees).toContain(320);
    expect(degrees).toContain(0);
    expect(degrees).toContain(40);

    // Offsets stay monotonic across the wrap, which is what stops the tape
    // jumping when the vehicle turns through north.
    const offsets = ticks.map((tick) => tick.offset);
    const ascending = offsets.every(
      (offset, i) => i === 0 || offset > (offsets[i - 1] ?? Number.NEGATIVE_INFINITY),
    );

    expect(ascending).toBe(true);
  });

  it('marks the cardinal and intercardinal ticks as major', () => {
    const ticks = headingTicks(0, 360, 10);

    expect(ticks.find((tick) => tick.deg === 0)?.major).toBe(true);
    expect(ticks.find((tick) => tick.deg === 90)?.major).toBe(true);
    expect(ticks.find((tick) => tick.deg === 10)?.major).toBe(false);
  });
});

describe('polar', () => {
  it('puts zero degrees at the top of the circle', () => {
    const [x, y] = polar(0, 0, 10, 0);

    expect(x).toBeCloseTo(0);
    expect(y).toBeCloseTo(-10);
  });

  it('advances clockwise', () => {
    const [x, y] = polar(0, 0, 10, 90);

    expect(x).toBeCloseTo(10);
    expect(y).toBeCloseTo(0);
  });
});

describe('formatting', () => {
  it('renders an absent value as the no-value marker, never as zero', () => {
    expect(num(null)).toBe(NO_VALUE);
    expect(num(undefined)).toBe(NO_VALUE);
    expect(num(Number.NaN)).toBe(NO_VALUE);
    expect(signed(null)).toBe(NO_VALUE);
    expect(bearing(null)).toBe(NO_VALUE);
  });

  it('renders a real zero as zero', () => {
    expect(num(0)).toBe('0.0');
    expect(signed(0)).toBe('+0.0');
    expect(bearing(0)).toBe('000');
  });

  it('signs climb rates the way an operator reads them', () => {
    expect(signed(1.25)).toBe('+1.3');
    expect(signed(-1.25)).toBe('-1.3');
  });

  it('pads and wraps bearings onto the compass card', () => {
    expect(bearing(7)).toBe('007');
    expect(bearing(359.6)).toBe('000');
    expect(bearing(-90)).toBe('270');
  });

  it('names only the cardinal points', () => {
    expect(cardinal(0)).toBe('N');
    expect(cardinal(270)).toBe('W');
    expect(cardinal(45)).toBeNull();
  });

  it('scales ages from sub-second to minutes', () => {
    expect(age(400)).toBe('0.4s');
    expect(age(12_000)).toBe('12s');
    expect(age(180_000)).toBe('3m');
    expect(age(null)).toBe(NO_VALUE);
  });
});
