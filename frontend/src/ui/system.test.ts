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

import { readFileSync, readdirSync } from 'node:fs';
import ts from 'typescript';
import { MISSION_ROUTE_COLOR, PREDICTION_COLOR, TRACK_COLOR, PANEL_COLOR, PANEL_DEEP_COLOR } from './palette';
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
  /^(font|font-size|font-weight|letter-spacing|line-height|padding|padding-[a-z]+|margin|margin-[a-z]+|gap|row-gap|column-gap|border-radius|border|border-[a-z]+|outline|outline-offset|box-shadow|top|left|right|bottom|width|height|min-[a-z]+|max-[a-z]+|flex-basis)$/;

const COLOR_LITERAL = /#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(/;

describe('design system boundary', () => {
  it('defines every colour in one place', () => {
    const offenders = declarations(DISPLAY).filter((line) => COLOR_LITERAL.test(line));

    expect(offenders).toEqual([]);
  });

  it('keeps the design scale out of instrument styling', () => {
    const scaleCSS = DISPLAY.replace(/[^{}]+\{([^{}]*)\}/g, (block: string, body: string) =>
      body.includes('design-geometry:')
        ? block.replace(/(?:min-|max-)?(?:width|height):[^;]+;/g, '') : block);
    const offenders = declarations(scaleCSS).filter((line) => {
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

/** Production TSX only: fixtures/assertions are not rendered application vocabulary. */
function tsxFiles(dir = new URL('../', import.meta.url)): { name: string; source: string }[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === 'gen') return [];
    const url = new URL(entry.name + (entry.isDirectory() ? '/' : ''), dir);
    if (entry.isDirectory()) return tsxFiles(url);
    return entry.name.endsWith('.tsx') && !entry.name.endsWith('.test.tsx')
      ? [{ name: url.pathname, source: readFileSync(url, 'utf8') }] : [];
  });
}

const TSX = tsxFiles();

describe('component vocabulary boundary', () => {
  it('loads tokens and primitives before feature CSS in Storybook', () => {
    const preview = readFileSync(new URL('../../.storybook/preview.tsx', import.meta.url), 'utf8');
    const system = preview.indexOf("import '../src/ui/system.css'");
    const display = preview.indexOf("import '../src/ui/display.css'");
    expect(system, 'Storybook must render the same design system as the app').toBeGreaterThanOrEqual(0);
    expect(display).toBeGreaterThan(system);
  });

  it('keeps color literals out of production TSX (DESIGN.md Colors)', () => {
    expect(TSX.filter(({ source }) => COLOR_LITERAL.test(source)).map(({ name }) => name)).toEqual([]);
  });

  it('uses Lever for buttons (DESIGN.md Buttons)', () => {
    const offenders = TSX.filter(({ name }) => !name.endsWith('/primitives.tsx')).flatMap(({ name, source }) => {
      const problems = [];
      if (/<button\b/.test(source)) problems.push(name);
      for (const match of source.matchAll(/document\.createElement\(['"]button['"]\)/g)) {
        const prefix = source.slice(Math.max(0, match.index - 160), match.index);
        if (!prefix.includes('design-exemption: imperative MapLibre marker uses the lever class.') || !source.includes('fleet-marker lever')) problems.push(name);
      }
      return problems;
    });
    expect(offenders).toEqual([]);
  });

  it('rejects fontSize attributes and exact legacy panel/label classes (DESIGN.md Four Steps and Components)', () => {
    const offenders: string[] = [];
    for (const { name, source } of TSX) {
      const tree = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
      const visit = (node: ts.Node) => {
        if (ts.isJsxAttribute(node)) {
          const attribute = node.name.getText(tree);
          if (attribute === 'fontSize') offenders.push(`${name}: fontSize belongs in token CSS`);
          if (attribute === 'className' && !name.endsWith('/primitives.tsx') && !name.endsWith('/Workspace.tsx')) {
            const classes = node.initializer?.getText(tree) ?? '';
            if (/(?:^|[\s"'`])(?:panel|label)(?=[\s"'`]|$)/.test(classes)) offenders.push(`${name}: use Group`);
          }
        }
        ts.forEachChild(node, visit);
      };
      visit(tree);
    }
    expect(offenders).toEqual([]);
  });

  it('pins MapLibre mirrors to CSS tokens (DESIGN.md Colors)', () => {
    for (const [name, value] of Object.entries({ 'mission-route': MISSION_ROUTE_COLOR, prediction: PREDICTION_COLOR, track: TRACK_COLOR, panel: PANEL_COLOR, 'panel-deep': PANEL_DEEP_COLOR })) {
      expect(SYSTEM).toContain(`--${name}: ${value};`);
    }
  });

  it('permits only the two named shadow uses (DESIGN.md Only-Overlays-Float)', () => {
    expect(declarations(DISPLAY).filter((line) => line.startsWith('box-shadow:'))).toEqual([]);
    expect(declarations(SYSTEM).filter((line) => line.startsWith('box-shadow:')).sort()).toEqual([
      'box-shadow: var(--overlay-ambient)', 'box-shadow: var(--selection-edge)',
    ]);
    expect(SYSTEM).toMatch(/\.views__popover\s*\{[^}]*box-shadow: var\(--overlay-ambient\)/);
    expect(SYSTEM).toMatch(/\.fleet-card\[data-selected="true"\]\s*\{[^}]*box-shadow: var\(--selection-edge\)/);
  });
});
