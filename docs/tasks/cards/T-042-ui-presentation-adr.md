---
id: T-042
title: Bind semantic rank to perceptual distance and gate the presentation system
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The display had a design system — four type steps, seven spacing tokens, one
button shape — and still read as unconsidered. Two audits and the T-014 review
captures located the cause, and it was not the token values.

Nothing bound semantic rank to perceptual distance. `.group__label` and
`.row__label` were both 10px uppercase `--lume-dim` separated only by weight and
tracking, and `.group__head` stood exactly as tall as a row, so an eight-group
rail rendered as one wall of grey text. `.row__note` already used the dimmer
`--absent` and still read as a second label, because `#7d8794` against `#8b939e`
is 0.79 contrast points.

`system.test.ts`'s scale gate read `display.css` only, so the file defining the
scale was the one file exempt from enforcing it: eight raw spacing literals, six
raw tracking literals — one a verbatim copy of `--track-label` — and six
ungoverned line-heights, plus two type values `display.css` introduced through
unitless `font:` shorthand leadings that the literal regex could not see.

Levers had no placement law: five of eight clusters reimplemented `LeverRow` in
`display.css` with three different paddings, `wide` was never used while
`display.css` faked it with `flex: 1 0 auto`, labels used two casings via two
mechanisms, and the command and mission groups arranged the same "control plus
its status" shape as mirror images of each other.

## Outcome

ADR 0005 is the binding presentation constitution, `system.test.ts` enforces it
mechanically, and the shipped surfaces conform.

## Scope

- In: the primitive vocabulary, the rail, the instrument column, the map
  overlay, the two bars, the developer tier, the gates, and the records.
- Out: fleet registry composition and the roster's hand-rolled rows (T-039);
  composition and information architecture, which stay deliberately unsettled;
  any new colour, type step, or capability.

## Acceptance criteria

- [x] A section header is a filled bar one row unit tall and cannot be mistaken for a row.
- [x] The row note is subordinated structurally — data font, indented, lowercase — not by colour alone.
- [x] Every data row is one `--row-h`; a noted row is exactly two.
- [x] Every lever is inline in a `LeverRow` or `wide`, with status prose above it and sentence-case source labels uppercased by CSS.
- [x] `caution` is reserved for aircraft hazard; the one `globalThis.confirm()` dialog is replaced by `Confirm`.
- [x] The scale gate reads `system.css`, catches unitless and shorthand leadings, and both stylesheets are clean.
- [x] Four `--fs-*` steps pair to four `--lh-*` leadings and two `--track-*` values, each asserted by count.
- [x] Every content voice clears 4.5:1 on all three panel surfaces, not one pair.
- [x] `display.css` cannot re-derive a primitive's geometry, with a narrow exemption naming T-039.
- [x] ADR 0005 written and indexed; DESIGN.md describes what shipped.

## Verification

```sh
cd frontend && pnpm test && pnpm lint && pnpm typecheck && pnpm build-storybook
cd frontend && pnpm dev --port 3001
SHOOT_OUT=frontend/.impeccable/review node scripts/shoot.mjs
```

Browser: mock fleet, open a vehicle, switch vehicles, toggle each panel, refresh
the mission, navigate the developer tabs by keyboard, and confirm a recording
delete through `Confirm` rather than a browser dialog.

## Open questions

None blocking. Two accepted consequences are recorded in ADR 0005: `--fs-lead`
is justified only while it is used, and the fleet roster's hand-rolled rows ship
under an exemption naming T-039.

## Notes

`scripts/shoot.mjs` needed two changes and they are not incidental: the click
helper compared `button.innerText` against the source string, which stops
matching once CSS uppercases lever labels, and the warm-up predicate read
`.group__note`, which is now `.group__annotation`.

The `Group.note` → `Group.annotation` rename is the reason for the second one.
`Group.note` was a right-aligned header annotation and `Row.note` a dimmer line
under a label — one prop name for two opposite things in two opposite places.
