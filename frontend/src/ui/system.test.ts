/**
 * Enforces the design system's boundary mechanically.
 *
 * `display.css` says in its own header that it may not introduce a colour, a
 * font size, or a spacing value — but a comment does not stop anyone, and the
 * previous system drifted into ten spacing values and six font sizes precisely
 * because nothing checked. This is the check. It runs in the ordinary test
 * gate, so drift fails CI rather than being noticed a month later in a
 * screenshot.
 *
 * Geometry is deliberately exempt. `width: 20px` on a mission marker is sized
 * to the radius of a MapLibre circle layer, not to a design scale, and forcing
 * it through a token would invent a token that means nothing.
 */

import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const SYSTEM = readFileSync(new URL('./system.css', import.meta.url), 'utf8');
const DISPLAY = readFileSync(new URL('./display.css', import.meta.url), 'utf8');

/** Strips comments so prose about colours is not mistaken for a declaration. */
function declarations(css: string): string[] {
  return css
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .split(/[;{}]/)
    .map((line) => line.trim())
    .filter((line) => line.includes(':'));
}

/** Properties whose values carry the design scale rather than geometry. */
const SCALE_PROPERTIES =
  /^(font|font-size|font-weight|letter-spacing|line-height|padding|padding-[a-z]+|margin|margin-[a-z]+|gap|row-gap|column-gap|border-radius)$/;

const COLOR_LITERAL = /#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(/;

describe('design system boundary', () => {
  it('defines every colour in one place', () => {
    const offenders = declarations(DISPLAY).filter((line) => COLOR_LITERAL.test(line));

    expect(offenders).toEqual([]);
  });

  it('keeps the design scale out of instrument styling', () => {
    const offenders = declarations(DISPLAY).filter((line) => {
      const [property = '', ...rest] = line.split(':');
      const value = rest.join(':');

      if (!SCALE_PROPERTIES.test(property.trim())) {
        return false;
      }

      // A bare `0`, a unitless line-height, and any var() are all fine.
      const literals = value.match(/(?<![\w-])\d*\.?\d+(px|rem|em)\b/g);

      return literals !== null && literals.length > 0;
    });

    expect(offenders).toEqual([]);
  });

  it('keeps the type scale at four steps', () => {
    const steps = SYSTEM.match(/^\s*--fs-[a-z]+:/gm) ?? [];

    // A fifth step is a decision someone has to make at every call site, which
    // is how the last system ended up with six font sizes that meant nothing.
    expect(steps).toHaveLength(4);
  });

  it('spends amber on caution only', () => {
    const offenders = declarations(SYSTEM)
      .concat(declarations(DISPLAY))
      .filter((line) => line.includes('var(--caution)'))
      .filter((line) => !/^(color|background|background-color|border|border-[a-z-]*color|fill|stroke)\b/.test(line.trim()));

    expect(offenders).toEqual([]);
  });

  it('never reintroduces a "good" green', () => {
    // Normal flight is unremarkable. A panel that lights up to say nothing is
    // wrong trains an operator to ignore it.
    expect(SYSTEM).not.toMatch(/--(ok|good|success|healthy|nominal)\b/);
  });

  it('keeps the absent-value colour above the contrast floor', () => {
    // `--absent` marks a value an operator has to read and act on, so it is
    // content and owes 4.5:1 on `--panel`. `--dead` is disabled chrome, which
    // WCAG exempts, and must not be used for content.
    const token = (name: string) => {
      const match = new RegExp(`--${name}: (#[0-9a-fA-F]{6})`).exec(SYSTEM);
      if (match?.[1] === undefined) throw new Error(`--${name} is not defined`);
      return match[1];
    };

    expect(contrast(token('absent'), token('panel'))).toBeGreaterThanOrEqual(4.5);
  });
});

/** WCAG 2.1 relative-luminance contrast ratio. */
function contrast(a: string, b: string): number {
  const lighter = Math.max(luminance(a), luminance(b));
  const darker = Math.min(luminance(a), luminance(b));

  return (lighter + 0.05) / (darker + 0.05);
}

function luminance(hex: string): number {
  const [r = 0, g = 0, b = 0] = [1, 3, 5].map((offset) => {
    const channel = Number.parseInt(hex.slice(offset, offset + 2), 16) / 255;

    return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });

  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
