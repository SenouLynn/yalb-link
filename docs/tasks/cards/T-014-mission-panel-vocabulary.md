---
id: T-014
title: Bring the mission panel into the display's design vocabulary
status: ready
priority: 2
owner: unassigned
depends_on: none
---

## Motivation and evidence

`display.css` documents a deliberate instrument palette — graphite bezel, the
blue-over-ochre attitude split, amber as the only warning colour, and no "good"
green, because a panel that lights up to say nothing is wrong trains an operator
to ignore it. Thirteen tokens carry it, and `Readout`, `Provenance`, `.panel`,
`.label`, `.value` and `.unit` apply it.

The mission panel added by T-007 uses none of that:

- `#c7f0ff` is hardcoded in six places across `display.css` and `MapPanel.tsx`
  and is not a palette token. T-010's map marker CSS copied it rather than
  questioning it.
- No provenance at all, though `MissionSnapshot.observed_at` is populated and
  every instrument beside it names its source message and age.
- A bare `<ol>` that dumps wire values: `params [15, 0, 0, 0] · autocontinue yes`.
- `.mission-list__active` needs an `!important`; nothing else in the sheet does.

## Outcome

The mission panel reads as part of the same instrument, and its snapshot carries
provenance like every other reading.

## Scope

- In: palette tokens, reuse of the existing readout/provenance components,
  typography, item presentation, and removing the `!important`.
- Out: layout and density rework and the message log (T-013), map viewport
  behaviour (T-012), and any change to what the mission download fetches.

## Acceptance criteria

- [ ] No hardcoded colour remains in the mission panel or its map layers; the
      commanded-route colour is a named token with a stated rationale.
- [ ] The snapshot's `observed_at` is shown as provenance in the established
      form.
- [ ] Item fields are presented rather than dumped; raw parameter arrays do not
      appear as bare arrays.
- [ ] The stylesheet needs no `!important`.
- [ ] Existing mission tests still pass unchanged in intent.

## Verification

```sh
cd frontend && pnpm typecheck && pnpm lint
cd frontend && pnpm vitest run src/mission src/map src/ui
cd frontend && pnpm dev   # /?source=mock, download, compare against the HUD above it
```

## Open questions

None blocking. If T-013's layout rework lands first it may absorb this card;
until then this is independently mergeable and does not depend on the reference
artifact being available.

## Notes

Split out of T-013 deliberately: this is unblocked and small, whereas parity
with flight-hud-trajectory cannot start until that reference is supplied.

See [[yalb-ui-emulate-flight-hud]] in operator memory.
