/**
 * Enforces the design system's boundary mechanically. See ADR 0005.
 *
 * `display.css` says in its own header that it may not introduce a colour, a
 * font size, or a spacing value — but a comment does not stop anyone, and the
 * previous system drifted into ten spacing values and six font sizes precisely
 * because nothing checked. This is the check. It runs in the ordinary test
 * gate, so drift fails CI rather than being noticed a month later in a
 * screenshot.
 *
 * The scale gate reads `system.css` as well, which it did not always do. While
 * only `display.css` was checked, the file that DEFINES the scale was the one
 * file exempt from enforcing it, and it accumulated eight raw spacing literals,
 * six raw tracking literals — one of them a verbatim copy of `--track-label`
 * sitting a few hundred lines below the comment warning against exactly that —
 * and six ungoverned line-heights. A gate that does not read the system is not
 * a gate on the system.
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

  /**
   * Every scale value comes from a token, in both stylesheets.
   *
   * A `design-geometry:` comment inside a block exempts that block's sizing and
   * `font` shorthand — a 20px marker box and a glyph centred in it are drawn
   * dimensions, not steps on a scale.
   */
  function scaleOffenders(css: string): string[] {
    const exempted = css.replace(/[^{}]+\{([^{}]*)\}/g, (block: string, body: string) =>
      body.includes('design-geometry:')
        ? block.replace(/(?:min-|max-)?(?:width|height):[^;]+;/g, '').replace(/font:[^;]+;/g, '') : block);

    return declarations(exempted).filter((line) => {
      const [property = '', ...rest] = line.split(':');
      const name = property.trim();
      const value = rest.join(':');

      // A custom property's declaration IS the token. That is where a literal belongs.
      if (name.startsWith('--') || !SCALE_PROPERTIES.test(name)) {
        return false;
      }

      // A bare `0` and any var() are fine; a dimensioned literal is not.
      if (/(?<![\w-])\d*\.?\d+(px|rem|em)\b/.test(value)) {
        return true;
      }

      /*
       * Leading is the half of the type system that used to escape this check.
       * A unitless `line-height: 1.6` and a `font: …/1.6 …` shorthand carry no
       * unit, so the regex above never saw them — which is how `display.css`
       * introduced two new type values while passing its own gate.
       */
      if (name === 'line-height' && /^\s*[\d.]+\s*$/.test(value)) {
        return true;
      }

      return name === 'font' && /\/\s*[\d.]+/.test(value);
    });
  }

  it('keeps the design scale out of instrument styling', () => {
    expect(scaleOffenders(DISPLAY)).toEqual([]);
  });

  it('keeps the design scale out of the system that defines it', () => {
    expect(scaleOffenders(SYSTEM)).toEqual([]);
  });

  it('keeps the type scale at four steps, each with one leading', () => {
    // `[a-z-]+`, not `[a-z]+`: the narrower pattern let `--fs-x-lead` add a
    // fifth step without the count ever seeing it.
    const steps = SYSTEM.match(/^\s*--fs-[a-z-]+:/gm) ?? [];
    const leadings = SYSTEM.match(/^\s*--lh-[a-z-]+:/gm) ?? [];

    // A fifth step is a decision someone has to make at every call site, which
    // is how the last system ended up with six font sizes that meant nothing.
    expect(steps).toHaveLength(4);

    // A step is a size AND a leading. Six unrelated line-heights across four
    // sizes is the same drift wearing a property name nobody was counting.
    expect(leadings).toHaveLength(steps.length);
  });

  it('keeps tracking to the two named steps', () => {
    // `.views__tier` once carried a literal `0.14em` — a second copy of
    // `--track-label` in the same file whose comment warns against exactly that.
    expect(SYSTEM.match(/^\s*--track-[a-z-]+:/gm) ?? []).toHaveLength(2);
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

  it('keeps every content voice above the contrast floor', () => {
    /*
     * The rank ladder is three content voices — the value, the label, and the
     * subordinate line under it — and all three are things an operator reads
     * and acts on, so all three owe 4.5:1 on every surface they sit on.
     *
     * Only `--absent` on `--panel` used to be checked, which left `--lume-dim`
     * — the colour of very nearly every label on the screen — unverified.
     *
     * `--dead` is deliberately absent from this list. It is disabled chrome,
     * which WCAG exempts, and it must never be used for content.
     */
    const token = (name: string) => {
      const match = new RegExp(`--${name}: (#[0-9a-fA-F]{6})`).exec(SYSTEM);
      if (match?.[1] === undefined) throw new Error(`--${name} is not defined`);
      return match[1];
    };

    const failures: string[] = [];
    for (const surface of ['panel', 'panel-deep', 'panel-raised']) {
      for (const voice of ['lume', 'lume-dim', 'absent', 'caution']) {
        const ratio = contrast(token(voice), token(surface));
        if (ratio < 4.5) failures.push(`--${voice} on --${surface} is ${ratio.toFixed(2)}:1`);
      }
    }

    expect(failures).toEqual([]);
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

  it('writes lever labels in sentence case (ADR 0005 The Lever Law)', () => {
    /*
     * `.lever` uppercases its label, the same way `Group` and `Row` labels are
     * uppercased. Screaming the string at the call site produces the identical
     * pixels by a second mechanism, and the app had one panel written each way.
     */
    const offenders: string[] = [];
    for (const { name, source } of TSX) {
      const tree = ts.createSourceFile(name, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
      const visit = (node: ts.Node) => {
        if (ts.isJsxElement(node) && node.openingElement.tagName.getText(tree) === 'Lever') {
          for (const child of node.children) {
            const text = child.getText(tree).trim();
            // Two or more consecutive capitals, with no lowercase anywhere.
            if (/[A-Z]{2,}/.test(text) && !/[a-z]/.test(text)) {
              offenders.push(`${name}: lever label "${text}" is uppercased at the call site`);
            }
          }
        }
        ts.forEachChild(node, visit);
      };
      visit(tree);
    }
    expect(offenders).toEqual([]);
  });

  it('keeps feature CSS from re-deriving a primitive (ADR 0005 One Implementation)', () => {
    /*
     * `display.css` may recolour a primitive's element — the active mission item
     * takes the route's colour — but it may not restate its geometry. Every
     * hand-rolled copy of `.row`, `.group__head` or `.lever-row` in this file
     * was a second implementation of a shape the vocabulary already owned, and
     * they drifted apart: five containers for "a run of levers", with three
     * different paddings between them.
     */
    const GEOMETRY = /^(display|grid-template-columns|flex|flex-basis|flex-direction|gap|row-gap|column-gap|padding|padding-[a-z]+|margin|margin-[a-z]+|width|height|min-[a-z]+|max-[a-z]+|position|align-items|justify-content)$/;
    const PRIMITIVE = /\.(group__|row__|lever|chip|note|field__|tabs?\b)/;

    const offenders: string[] = [];
    // Comments first: prose naming `.chip` is not a selector matching one.
    const rules = DISPLAY.replace(/\/\*[\s\S]*?\*\//g, '');
    for (const [, selector = '', body = ''] of rules.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      // T-039 owns replacing the fleet roster's hand-rolled rows with `Row`.
      if (selector.includes('.fleet-card')) continue;
      if (!PRIMITIVE.test(selector)) continue;

      for (const line of declarations(body)) {
        const [property = ''] = line.split(':');
        if (GEOMETRY.test(property.trim())) {
          offenders.push(`${selector.trim()} { ${line} }`);
        }
      }
    }
    expect(offenders).toEqual([]);
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
