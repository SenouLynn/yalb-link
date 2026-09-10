---
name: YALB-GCS
description: A ground control station whose screen is a declaration of the telemetry stream, not a composed page.
colors:
  panel: "#14171c"
  panel-deep: "#0d1013"
  panel-raised: "#1b1f26"
  bezel: "#272c34"
  lume: "#d8dde3"
  lume-dim: "#8b939e"
  caution: "#d9a441"
  absent: "#7d8794"
  dead: "#4d5560"
  sky: "#2d6d9e"
  ground: "#7a5326"
  prediction: "#9fb4c9"
  mission-route: "#c7f0ff"
  track: "#b6a6d9"
typography:
  lead:
    fontFamily: "ui-monospace, \"SF Mono\", \"SFMono-Regular\", Menlo, Consolas, monospace"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.15
    letterSpacing: "normal"
    fontVariation: "tabular-nums"
  body:
    fontFamily: "ui-monospace, \"SF Mono\", \"SFMono-Regular\", Menlo, Consolas, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0.06em"
  data:
    fontFamily: "ui-monospace, \"SF Mono\", \"SFMono-Regular\", Menlo, Consolas, monospace"
    fontSize: "0.6875rem"
    fontWeight: 400
    lineHeight: 1.25
    letterSpacing: "0.06em"
    fontVariation: "tabular-nums"
  label:
    fontFamily: "ui-sans-serif, system-ui, \"Helvetica Neue\", Arial, sans-serif"
    fontSize: "0.625rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.14em"
rounded:
  hairline: "2px"
spacing:
  row-y: "0.0625rem"
  row-x: "0.5rem"
  row-h: "1.25rem"
  target-h: "1.75rem"
  target-x: "0.625rem"
  lever-gap: "0.25rem"
  pad-chip: "0.3rem"
components:
  lever:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.lume}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 0.625rem"
    height: "1.75rem"
  lever-hover:
    backgroundColor: "{colors.panel-raised}"
    textColor: "{colors.lume}"
  lever-active:
    backgroundColor: "{colors.panel-deep}"
    textColor: "{colors.lume}"
  lever-caution:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.caution}"
    rounded: "{rounded.hairline}"
    height: "1.75rem"
  lever-disabled:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.absent}"
  lever-pressed:
    backgroundColor: "{colors.panel-deep}"
    textColor: "{colors.lume}"
  field-input:
    backgroundColor: "{colors.panel-deep}"
    textColor: "{colors.lume}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 0.625rem"
    height: "1.75rem"
  field-input-disabled:
    backgroundColor: "{colors.panel-deep}"
    textColor: "{colors.dead}"
  row:
    backgroundColor: "transparent"
    textColor: "{colors.lume}"
    typography: "{typography.data}"
    padding: "0.0625rem 0.5rem"
  row-label:
    textColor: "{colors.lume-dim}"
    typography: "{typography.label}"
  row-absent:
    textColor: "{colors.absent}"
    typography: "{typography.data}"
  row-caution:
    textColor: "{colors.caution}"
    typography: "{typography.data}"
  group-head:
    backgroundColor: "lume 7% wash"
    textColor: "{colors.lume-dim}"
    typography: "{typography.label}"
    padding: "0 0.5rem"
    height: "1.25rem"
  tab:
    backgroundColor: "transparent"
    textColor: "{colors.lume-dim}"
    typography: "{typography.label}"
    padding: "0 0.625rem"
    height: "1.75rem"
  tab-selected:
    backgroundColor: "transparent"
    textColor: "{colors.lume}"
  chip:
    backgroundColor: "transparent"
    textColor: "{colors.lume-dim}"
    typography: "{typography.data}"
    rounded: "{rounded.hairline}"
    padding: "0 0.3rem"
  chip-active:
    textColor: "{colors.lume}"
  chip-caution:
    textColor: "{colors.caution}"
  app-bar:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.lume}"
    typography: "{typography.label}"
    padding: "0 0.5rem"
    height: "1.75rem"
  view-bar:
    backgroundColor: "{colors.panel-deep}"
    textColor: "{colors.lume-dim}"
    typography: "{typography.label}"
    padding: "0 0.5rem"
    height: "1.75rem"
---

# Design System: YALB-GCS

## Overview

**Creative North Star: "The Instrument Bezel"**

The screen is a declaration of the telemetry stream, not a composed page. Flat graphite panels butt against each other on 1px bezel rules — no gaps, no cards, no shadows, nothing centered in void. Every number is a labelled row that names the MAVLink message it came from and how old that message is. Whitespace that carries no information is screen area stolen from data, so the density is closer to an avionics panel or a DearImGui debug window than to a dashboard.

The palette is taken from instrument hardware rather than from a generic dark theme: a graphite bezel, the blue-over-ochre split of a real attitude indicator, off-white engraved lettering, and amber as the only warning colour. There is deliberately no "good" green under any name. Normal flight is unremarkable, and a panel that lights up to say nothing is wrong trains an operator to ignore it — the test gate asserts no `--ok`, `--good`, `--success`, `--healthy`, or `--nominal` token can ever be reintroduced.

Two axes govern size, and they are independent on purpose. The density axis (`--row-*`) sizes data and goes as tight as it reads, on one unit — `--row-h` — so a column has a rhythm rather than whatever height its type happened to sum to. The target axis (`--target-*`) sizes anything a pointer has to hit and stays generous regardless, because field operation on a laptop is expected and a control sized off the data scale would be unhittable there. Tight data, generous levers.

**Key Characteristics:**
- Panels butted edge to edge on 1px rules; no gaps, no floating cards, no drop shadows on content
- Every section announces itself with a filled header bar one row unit tall
- Every reading states its source and age over a rule that drains toward its TTL
- Four fixed type steps, four leadings, and no fifth of either
- Data columns are built on one row unit: a row is one, a noted row is two
- Monospace tabular values right-aligned against a shared edge
- Amber for caution only; no green for "good"
- One button shape and two placements in the entire application

## Colors

An instrument-hardware palette: graphite greys, off-white engraved lettering, and exactly one warning hue.

### Primary
- **Lume** (`#d8dde3`): every value, every active label, the focus ring. Off-white engraved lettering on graphite. This is the colour of information that is present and current.
- **Lume Dim** (`#8b939e`): row labels, units, section headers, provenance text, the drained-bar fill. The subordinate voice — everything that frames a value rather than being one.

### Secondary
- **Caution Amber** (`#d9a441`): a value out of tolerance, a stale provenance bar, the arm/disarm lever's border, the replay transport's frame. Never chrome, never a focus ring, never a fill behind text.

### Tertiary
- **Instrument Sky** (`#2d6d9e`) and **Instrument Ground** (`#7a5326`): the two halves of the artificial horizon, and nothing else.
- **Prediction Blue-Grey** (`#9fb4c9`): the five-second projected path on the map and its legend chip. Deliberately not amber — a prediction is not a hazard.
- **Mission Cyan** (`#c7f0ff`): the commanded route, its point markers and its legend chip. Distinct from both the flown track and the caution amber. Mirrored as a resolved literal in `src/ui/palette.ts` because MapLibre cannot read custom properties.

- **Track Lavender** (`#b6a6d9`): solid flown history, separating observed travel from the pale cyan commanded mission and dashed blue-grey prediction. Mirrored for MapLibre and pinned by the token gate.

### Neutral
- **Panel Graphite** (`#14171c`): the working surface of every panel and the app bar.
- **Panel Deep** (`#0d1013`): the page beneath, the view subbar, input wells, and a pressed lever.
- **Panel Raised** (`#1b1f26`): hover only — a lever, a tab, a popover option under the pointer.
- **Bezel** (`#272c34`): every 1px rule, every control border at rest, the empty provenance track. The structure of the screen is drawn entirely in this one grey.
- **Absent Grey** (`#7d8794`): a value that is genuinely missing — a dash the operator has to read and act on. That is content, so it clears 4.5:1 on Panel Graphite (5.0:1, asserted in `src/ui/system.test.ts`).
- **Dead Grey** (`#4d5560`): disabled chrome only — a disabled border, a disabled input's text. Measures 2.38:1, which WCAG exempts for disabled controls.

### Named Rules
**The Amber-Means-Aircraft Rule.** Amber says something is wrong with the aircraft. It may only ever land on `color`, a `background`, a `border-color`, `fill`, or `stroke` — the test gate rejects any other property — and it may never be spent on chrome or on a focus ring. `--focus` is lume.

**The No-Good-Green Rule.** There is no green under any name. Normal is the absence of amber, never the presence of a reassuring colour.

**The Rank Ladder Rule.** Three content voices — Lume for a value, Lume Dim for the label that names it, Absent Grey for the subordinate line beneath — and all three owe 4.5:1 on every surface they sit on, because all three are read and acted on. `src/ui/system.test.ts` checks each against Panel Graphite, Panel Deep, and Panel Raised. **Rank is never carried by colour alone.** Colour is the weakest signal here: at 10px on graphite, two greys a few points apart are the same grey, and the row note proved it by being Absent Grey and still reading as a second label. A change of rank moves at least two of surface, typeface, indent, and case. See ADR 0005.

**The Two Absences Rule.** Disabled chrome is Dead Grey and is exempt from the contrast floor. A missing value is Absent Grey and is not — it is content and owes 4.5:1. Never use Dead Grey for something the operator has to read. A disabled lever is the exception that proves it: its border goes Dead, its label stays Absent, because the command lever is often the only control on screen and rendering it as the dimmest object on the page is bad regardless of what the exemption allows.

## Typography

**Data Font:** ui-monospace / SF Mono / Menlo / Consolas — every value, every control label, every piece of prose in a panel.
**Label Font:** ui-sans-serif / system-ui — section headers and row labels only.

**Character:** Engraved and mechanical. Values are monospace with `tabular-nums` throughout, because proportional digits make a readout jitter sideways as the numbers change and that reads as instability in the data. Labels are a letter-spaced uppercase sans at 10px, sized to name a column without competing with it. Nothing on this surface is larger than 17px; there is no display type, because there is no headline — the largest thing on screen is a number.

### Hierarchy
- **Lead** (mono, 17px, 1.15): at most one emphasised value per group — altitude above home, the headline of a table. One per group, not six.
- **Body** (mono, 12px, 1.5): lever and tab-adjacent control text, hints, prose in a panel, input text.
- **Data** (mono, 11px, 1.25, tabular): the value in a data row. The default size of the entire application.
- **Label** (sans, 10px, 600, 0.14em tracking, uppercase): section headers over hairline rules. Row labels use the same step at 0.06em tracking; provenance and units use it in the data font.

### Named Rules
**The Four Steps Rule.** Four type steps exist — 10px, 11px, 12px, 17px — and `src/ui/system.test.ts` asserts the count. A fifth step is a decision someone has to make at every call site, which is how the predecessor drifted into six unrelated font sizes.

**The Leading Rule.** A step is a size *and* a leading. Four `--lh-*` tokens pair to the four sizes and the gate asserts one per step, so a fifth leading cannot appear without a fifth step to hang it on. Four sizes carrying six unrelated line-heights is the Four Steps drift wearing a property name nobody was counting — and because a unitless `line-height` and a `font: …/1.6` shorthand carry no unit, the literal check had to be widened to see them at all. Tracking is two tokens for the same reason. See ADR 0005.

**The Fixed-DPI Rule.** Sizes are fixed rem, never fluid, never `clamp()`. An operator views this at one DPI, and a heading that shrinks inside a rail looks broken rather than responsive.

## Layout

Exactly one viewport; nothing scrolls the page. `html` and `body` are `overflow: hidden`, the shell is a `100dvh` grid of three rows — app bar, view subbar, body — and slots scroll internally. A body scrollbar would narrow the viewport, resize every column, and make the map jump.

The body is a **flex row of slots, not a named grid**. Four slot roles: `rail` (fixed 15rem context column of stacked labelled groups), `center` (flex `1 1 auto`, the thing being looked at, full bleed), `aux` (fixed 21rem instrument stack), and `dev` (a bounded 10rem tabbed developer tier inside the aux column). Slots are divided by a 1px right-hand rule, and the last one drops it. The predecessor hand-enumerated every visible-panel combination as a class (`.has-instruments.has-map.has-mission` and siblings); flex sizes whatever is mounted, which is what makes adding information cost one entry in `src/workspace/registry.ts` and zero layout CSS.

Spacing comes from seven tokens and nowhere else, and `src/ui/system.test.ts` now enforces that in `system.css` as well as in `display.css` — while only the latter was checked, the file defining the scale was the one file exempt from it, and it had collected eight raw spacing values and six raw tracking values. The density axis is `--row-y` (1px), `--row-x` (8px), and `--row-h` (20px). `--row-h` is the unit the data columns are built on: a plain row is one, a row carrying a note is exactly two, a group header bar is one. There is deliberately no group-separation token — groups butt directly and the header bar separates them, which is why the old `--group-y` is gone. The target axis is `--target-h` (28px minimum height for anything clickable), `--target-x` (10px horizontal padding on the same), and `--lever-gap` (4px between levers). `--pad-chip` (≈5px) is the chip's inline padding. Tracking (`0.14em` labels, `0.06em` data) is part of the scale too, because instrument styling reaches for it and a second copy of `0.14em` in another file is how a scale stops being one.

Responsive behaviour is **structural, not fluid**: at `max-width: 60rem` the flex row becomes a single scrolling column of bounded regions, slot rules move from right to bottom, and slot content caps at 34rem so a label and its right-hand value edge do not end up 700px apart (the map is exempt). Type never scales. Order is reassigned so flight state outranks the map on a phone: rail, then aux, then center — in DOM order the first viewport was the rail plus a slice of map, with no altitude, speed, attitude or battery above the fold.

### Named Rules
**The One Viewport Rule.** The page never scrolls. If content does not fit, a slot scrolls inside its own bounds.

**The Registry Rule.** Adding or removing information on screen is one registry entry plus a renderer. If a new panel needs layout CSS, the slot model is being worked around.

**The Hidden-Never-Unmounted Rule.** Panels are hidden with the `hidden` attribute and stay mounted. Unmounting tears down the MapLibre context and drops an in-flight mission download. `display: none` on a flex item leaves the layout identical to a slot that was never there, so there is no reason to unmount.

## Elevation & Depth

This system is flat. Depth is carried entirely by three tonal steps of graphite (Panel Deep beneath, Panel Graphite as the working surface, Panel Raised on hover) and by 1px Bezel rules. There are no drop shadows on content, no cards, and no gaps between panels — a panel inside a slot is just a run of rows, and panel chrome belongs to the slot. Two shadows exist in the entire build and neither lifts a surface: a 1px inset edge marking the selected fleet row, and an ambient shadow under the one true overlay.

### Shadow Vocabulary
- **Selection edge** (`box-shadow: inset 1px 0 0 var(--lume)`): the selected row in the fleet roster, paired with an 8% lume wash. Inset, so it reads as a marked edge rather than a lifted object.
- **Overlay ambient** (`box-shadow: 0 4px 14px rgb(0 0 0 / 55%)`): the Views popover, and only the Views popover. It is the sole element that genuinely floats above the plane.

### Named Rules
**The Butted-Panels Rule.** Surfaces meet on a hairline. Never a gap, never a card, never a radius above 2px, never a shadow to separate two things a rule already separates.

**The Only-Overlays-Float Rule.** A shadow is permission to float, and only a popover or a map overlay has it. If a resident panel needs a shadow to be legible, its tonal step is wrong.

## Shapes

Rectangular throughout. `--radius` is 2px and nothing exceeds it — controls, chips, popovers, and map markers all share that single almost-square corner, which reads as a machined edge rather than as a soft one. Structure is drawn in 1px: `--rule` (`1px solid var(--bezel)`) is the border of every control at rest, the divider between slots, the underline of every group header, and the 1px provenance track. There is no second border weight; a selected tab thickens its own bottom edge to 2px and that is the only exception.

Silhouettes recur: the two-column row, the header-rule-body group, the horizontal lever row, and the 1px drain bar. Each is a rectangle aligned to its neighbours' edges.

## Components

### Buttons
The lever. There is exactly one button shape in the application, rendered by `Lever` and styled once in `system.css` — so "the commit button looks different over here" cannot happen. Variants change colour and width, never form.

- **Shape:** near-square (`2px` radius), 1px Bezel border, 28px minimum height, 10px horizontal padding, 12px mono uppercase-agnostic label at weight 600.
- **Default:** Panel Graphite on the surface it sits on, Lume label.
- **Hover:** Panel Raised fill, border lifts to Lume Dim. **Active:** Panel Deep fill.
- **Focus:** 2px Lume outline at 1px offset. Never amber.
- **Pressed / toggled** (`aria-pressed="true"`): Panel Deep fill with a Lume Dim border — a control holding a position, not a second style.
- **Disabled:** Dead Grey border, Absent Grey label. The border carries "disabled"; the label stays readable.
- **Caution:** amber border and amber label, hovering to a 12% amber wash with a Lume label. Amber-edged because the action itself is the hazard — arming a vehicle — not because it is primary.
- **Wide:** full width, for a group's one committing action. Width is the only other axis.

**The Lever Law** (ADR 0005). Two placements and no third: inline in a `LeverRow` at content width, or `wide` filling the group. A lever is never floated beside its own status text — status prose sits *above* it as a `Note`, because a control and its status read as one thing stacked and as two unrelated things side by side. Labels are written in sentence case and uppercased by CSS, the same way `Group` and `Row` labels are; screaming the string at the call site produces identical pixels by a second mechanism. `caution` means the aircraft is the hazard; a destructive *data* action is guarded by `Confirm` instead, which is also why no `globalThis.confirm()` dialog remains in the application. A lever that changes the address takes `href` and renders an anchor rather than borrowing the lever class.

### Chips
- **Style:** inline-flex, 1px Bezel border, 2px radius, 10px mono at 0.06em tracking, Lume Dim, `--pad-chip` inline padding and no vertical padding, so it sits on the text baseline beside a label.
- **State:** `active` goes Lume with a Lume Dim border; `caution` goes amber on amber; `dead` goes Dead Grey. LOST uses `caution`; `dead` is reserved for disabled chrome. A chip carries a label's own state — rows carry values.

### Notes
`Note` is panel prose with `normal`, `caution`, and `absent` tones. Missing information remains readable in Absent Grey. Group empty states use Note internally.

### Group Headers
A filled bar — a 7% lume wash bled to the slot edge, one row unit tall, carrying the uppercase label left and the group's provenance right over a 1px bottom rule. It is ImGui's `CollapsingHeader`, and it exists because a header and a row label previously differed only by weight and tracking while standing the same height, so an eight-group rail read as one undifferentiated wall of text. The wash is additive rather than a fixed fill, so the same rule reads correctly on `--panel` in a slot and on `--panel-deep` inside the popover; `--panel-raised` is spoken for by hover and would make a resting header look like one the pointer is on.

### Cards / Containers
There are none. `Group` is a labelled section with `min-width: 0` and no border, background, or radius around its body. Every feature owns its Group and header provenance; the registry only mounts nodes. The container is the slot, and its only chrome is a 1px rule against its neighbour.

### Inputs / Fields
- **Field:** accepts either text input or a native select with labelled options.
- **Style:** a 10px-tracked label over a Panel Deep well with a 1px Bezel border, 2px radius, 28px minimum height, mono tabular text. Sized on the target axis, not the density axis.
- **Focus:** 2px Lume outline at 1px offset, border to Lume Dim.
- **Disabled:** Dead Grey text and border.
- **Confirm:** a 14px checkbox beside its assertion spelled out in full. A checkbox next to the word "confirm" attests to nothing.

### Navigation
Two stacked bars, each 28px minimum: the app bar in Panel Graphite carrying the 10px `0.16em` uppercase title left and the source badge right; the view subbar in Panel Deep beneath it, lighter because it is the subordinate of the two, carrying the back action and the Views control. Slot-level navigation is the tab strip: 10px `0.1em` uppercase Lume Dim tabs on a hairline, hovering to Lume on Panel Raised, selected by a 2px Lume bottom edge, with an optional right-aligned note (a count, a rate) pushed to the end of the strip. The Views popover is the one floating menu: Panel Graphite, 1px border, 2px radius, tier headings in the label step, options 28px tall on the target axis.

### The Data Row
The atom, and the reason most of this system exists. A CSS **grid** of `minmax(0, 1fr) auto` — not a flex row — with a 10px uppercase Lume Dim label left and an 11px mono tabular value right-aligned against a shared edge, unit dimmer and inline. The shared edge is the whole point: a column of rows must scan as one table the eye can run down, and values that each stop wherever their string ends read as separate objects that happen to be nearby. A 4% lume hover wash tells the pointer which row it is on. Every row is exactly one `--row-h`; a row carrying a note is exactly two, never one and a half and never three. Three modifiers, and no more: `lead` (the 17px step, once per group — it leads each instrument group, and a step with no consumer should be deleted rather than kept), `stacked` (value wraps beneath the label, left-aligned — for coordinates), and a `note` (a second line under the label carrying a convention the operator cannot infer, like "+ below target", on its own line so it cannot orphan a word beside the next row's number).

The note is subordinated **structurally, not by colour**: the data font against the label's sans, indented one `--row-x` past the label's left edge, lowercase, with Absent Grey as the last of four signals rather than the only one. It was already Absent Grey and still read as a second label, because 0.79 contrast points is not a rank at 10px on graphite. This is the general rule — see The Rank Ladder in ADR 0005: rank is never carried by colour alone, and a change of rank moves at least two of surface, typeface, indent, and case.

### Provenance
The signature of this display. A 1px Bezel track holding a Lume Dim fill that drains by `scaleX` transform as a value ages toward its TTL, over a 10px mono line naming the MAVLink source left and the age right. The drain is what makes staleness pre-attentive: the operator sees the bar emptying without reading the age. Stale turns fill and text amber but never paints a background, which would bury the source name written on it. Unavailable empties the fill and drops the text to Absent Grey. It is scaled rather than resized because every reading on screen holds one and they all redraw four times a second; animating `width` would put that many layout passes on the main thread each tick.

**The Once-Per-Group Rule.** Provenance is stated once on a group header, not once per row. On a header the drain track sits *beneath* its own text, against the bar's bottom edge; above it, the track read as a hairline floating over unrelated words rather than as a meter. Per-row provenance inserted a second right-aligned value at a different edge, so ten readings rendered as twenty alternating half-rows with `VFR_HUD` three times running — the exact opposite of the shared value edge the row exists to form.

## Do's and Don'ts

### Do:
- **Do** build every panel from the closed primitive set — `Group`, `Row`, `Lever`, `LeverRow`, `Field`, `Confirm`, `Tabs`, `Chip`, `Note`. If one cannot express a new panel, extend a primitive rather than styling the panel directly.
- **Do** keep the two axes separate: `--row-*` for data, `--target-*` for anything a pointer hits. Tight data, generous levers.
- **Do** right-align every value against the shared edge with `tabular-nums`, so a column of readings scans as one table.
- **Do** state provenance once per group header.
- **Do** put instrument-specific styling in `display.css` built only from `system.css` tokens; geometry (a marker's 20px box, a MapLibre circle radius) is the one exemption.
- **Do** say what is absent and why in the `absent` slot — never "no data", which an operator cannot act on.
- **Do** keep the default basemap a desaturated dark canvas (Esri Dark Gray Canvas), so the flown track, the prediction and the commanded route are the only saturated things on the map.
- **Do** hide panels with `hidden` and leave them mounted.

### Don't:
- **Don't** introduce a colour, a font size, or a spacing value in `display.css` or any panel stylesheet. `src/ui/system.test.ts` fails the build for it. If a panel needs one, the token is missing and belongs in `system.css`.
- **Don't** add a fifth type step, a fifth leading, or a third tracking value.
- **Don't** carry a change of rank on colour alone; move at least two of surface, typeface, indent, and case.
- **Don't** float a lever beside its own status text, or uppercase a lever label at the call site.
- **Don't** re-derive a primitive's geometry in `display.css`. Recolouring one of its elements is fine; restating its layout is a second implementation.
- **Don't** spend amber on chrome, on a focus ring, or on a background behind text; and don't introduce a green, or any token named `ok`, `good`, `success`, `healthy`, or `nominal`.
- **Don't** use Dead Grey for anything the operator has to read.
- **Don't** add a second button shape, or a third placement. A variant may change colour or width; never form.
- **Don't** spend `caution` on an action that is destructive to data rather than to the aircraft; guard that with `Confirm`.
- **Don't** give a resident surface a drop shadow, a card border, a background, or a radius above 2px. Only a popover or a map overlay floats.
- **Don't** put more than one `lead` value in a group.
- **Don't** make the page scroll, and don't nest a scroll region inside a region that already scrolls.
- **Don't** write layout CSS to add a panel; add a registry entry.
- **Don't** use fluid or `clamp()` type. Sizes are fixed at every viewport.

## Round-two validation

The production TSX gate rejects raw buttons, color literals, fontSize attributes,
and exact legacy `panel`/`label` classes. The imperative MapLibre marker has an
annotated Lever-class exemption. CSS geometry exceptions are explicit, palette
mirrors are tested, and only named selection/overlay shadows are permitted.

Developer tabs use roving focus (arrows, Home, End), linked tab/panel IDs and
resident hidden panels. Mock missions load on identity/source change; live and
replay still require an explicit download. Snapshot provenance identifies a
non-expiring observation, not proof that the onboard mission is unchanged.

Browser review: 1600×1000 and 560×900, generated by `node scripts/shoot.mjs`.
The stacked instrument capture confirms the aux/developer order and shared 8px
row inset. Source/viewport provenance accompanies `.impeccable/review/` captures.
Fleet registry/mount lifecycle work remains explicitly deferred under T-039.

Heartbeat state provenance uses the heartbeat's observed_at, with receipt-time
fallback only for legacy fixtures. The backend emits changes, not periodic
heartbeats, so an unchanged state has no telemetry TTL. LOST changes its
provenance to caution without resetting its observation age.
