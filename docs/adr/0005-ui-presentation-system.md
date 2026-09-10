# ADR 0005: The UI presentation system

## Context

The display has had a design system since T-014: four type steps, seven spacing
tokens, one button shape, and a test gate. It nevertheless read as unconsidered,
and the reason was not the token values. Two things were missing, and both are
costly to reverse once panels have been written against them.

The first is that **nothing bound semantic rank to perceptual distance**. A
section header and a row label were both 10px uppercase `--lume-dim` separated
only by weight and tracking, and a group header stood exactly as tall as one of
its own rows. An eight-group rail therefore rendered as a single wall of grey
text in which no section boundary was visible. The row note had already been
given the dimmer `--absent` and still read as a second label, because
`#7d8794` against `#8b939e` is 0.79 contrast points and the note otherwise
shared the label's font, size, and left edge.

The second is that **the gate did not read the file that defines the scale**.
`system.test.ts` checked `display.css` only, so the system's own stylesheet was
the one file exempt from enforcing the system. It accumulated eight raw spacing
literals, six raw tracking literals — one of them a verbatim copy of
`--track-label` a few hundred lines below the comment warning against exactly
that — and six ungoverned line-heights. `display.css` meanwhile introduced two
new type values through `font:` shorthand leadings, which carry no unit and so
were invisible to a regex that required one.

## Decision

Five rules govern presentation. They are stated here rather than in DESIGN.md
because DESIGN.md describes what shipped and is rewritten whenever it changes;
these are the constraints that decide what may ship.

### The Rank Ladder

There are three content voices and one chrome voice. `--lume` is a value,
`--lume-dim` is the label that names it, and `--absent` is the subordinate line
beneath — a sign convention, a unit, a provenance, or a value that is missing.
All three are read and acted on, so all three owe 4.5:1 on every surface they
appear on, and the gate now checks each against `--panel`, `--panel-deep`, and
`--panel-raised` rather than checking one pair. `--dead` is disabled chrome,
which WCAG exempts, and is never used for content.

**Rank is never carried by colour alone.** Colour is the weakest signal
available on this surface: at 10px on graphite, two greys a few points apart are
the same grey, and the field-legibility requirement makes that worse rather than
better. A change of rank moves at least two of surface, typeface, indent, and
case. The group header takes a surface — a lume wash bled to the slot edge, one
row unit tall, which is ImGui's CollapsingHeader and cannot be mistaken for a
row. The row note takes typeface, indent, and case: the data font, indented one
`--row-x` past the label, lowercase, with `--absent` as the last signal rather
than the only one.

The wash is additive rather than a fixed fill, so one header rule reads
correctly on `--panel` in a slot and on `--panel-deep` inside the popover.
`--panel-raised` is spoken for by hover and would make a resting header look
like one the pointer is on.

### The Row Unit

`--row-h` is 1.25rem and the data columns are built on it. A plain row is one
unit, a row carrying a note is exactly two, a group header bar is one. Before
the unit existed, a row's height was whatever its type happened to sum to — a
plain row landed near 19px and a noted row near 42px — so a column alternated
between two unrelated rhythms and nothing lined up with anything.

There is deliberately **no group-separation token**. Groups butt directly and
the header bar is the separator. The retired `--group-y` was spending vertical
space to say what a surface now says, and it was also numerically identical to
`--target-x` and `--fs-label`, which made the density and target axes
indistinguishable in the numbers even though the file argued they were separate.

### The Lever Law

One shape, and exactly two placements. A lever either sits inline in a
`LeverRow` at its content width, or it is `wide` and fills its group as that
group's one committing action. There is no third placement, and in particular a
lever is never floated beside its own status text: a control and its status read
as one thing stacked and as two unrelated things side by side. The command and
mission groups had previously arrived at mirror images of each other, one with
the lever left and its state right and one with the state left and its lever
right.

**Status prose sits above the lever, as a `Note`.** **Labels are written in
sentence case and uppercased by CSS**, the same way `Group` and `Row` labels
already were; screaming the string at the call site produces identical pixels by
a second mechanism, and the application had half its levers written each way.

`caution` means the aircraft is the hazard. Deleting a recording is destructive
to data, not to the vehicle, so it is not amber — spending amber there is how
amber stops meaning anything. Destructive data actions are guarded by `Confirm`,
which also removes the one `globalThis.confirm()` dialog in the application: a
browser modal carries no design system, cannot be styled, and names the thing it
is about to destroy in a different voice from the surface that raised it.

### The Leading Rule

Leading is a property of the type step, not of the call site. There are four
steps and four `--lh-*` tokens, and the gate asserts one leading per step, so a
fifth leading cannot appear without a fifth step to hang it on. Tracking is two
tokens for the same reason. Four sizes with six unrelated line-heights is the
same drift the Four Steps Rule was written against, wearing a property name
nobody had thought to count.

### The Gate Covers The System

`system.test.ts` reads `system.css` as well as `display.css`. Only a custom
property's own declaration may contain a literal — that declaration *is* the
token, and is where a number belongs. A block carrying a `design-geometry:`
comment is exempt for sizing and `font` shorthand, because a 20px marker box and
a glyph centred in it are drawn dimensions and not steps on a scale.

The literal check now also catches unitless `line-height` and the `font:`
shorthand's `/1.6` form. A gate that does not read the system is not a gate on
the system, and one that only sees dimensioned numbers is not a gate on type.

## Consequences

Adding a value to the screen should not require a design decision. That is what
`primitives.tsx` has claimed in its header comment since it was written; these
rules and the gates behind them are what make the claim true rather than
aspirational. The cost is that genuinely new presentation now requires either a
token or an annotated exemption, and neither can be added quietly.

Two consequences are accepted rather than solved. The `--fs-lead` step is
justified only while it is used — it had a single consumer application-wide,
which made a four-step scale really three steps and a special case; it now leads
each instrument group, and if that use is ever removed the step should go with
it. And the fleet roster still hand-rolls copies of `.row` and `.group__head`
in `display.css`; the re-derivation gate carries a narrow exemption naming
T-039, which owns replacing them, rather than silently tolerating them.

This ADR does not settle composition. What is on the screen, in what order, and
in which slot remains deliberately unsettled, and none of these rules decide it.

## Verification

`src/ui/system.test.ts` is the verification point. It fails the ordinary test
gate rather than being noticed a month later in a screenshot:

```sh
cd frontend && pnpm vitest run src/ui/system.test.ts
```

Visual confirmation is `node scripts/shoot.mjs` against `?source=mock` at
1600x1000 and 560x900, whose captures are the evidence that the header bar is
legible, that row heights are one or two units, and that every lever carries one
casing and one of two widths.
