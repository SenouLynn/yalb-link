---
id: T-013
title: Reach flight-hud-trajectory parity on the real backend
status: backlog
priority: 1
owner: unassigned
depends_on: T-015, T-016
---

## Motivation and evidence

The operator's benchmark for this display is the earlier **flight-hud-trajectory**
artifact: it had a message log, a substantially different layout, and far more
state on screen at once. That artifact is not in this repository's history and
must not be looked for there — this repository is the rebuild, whose point is to
reach that UI on a backend that actually works.

Assessed against it on 2026-09-08, the current display is thin. This is not a
styling gap and should not be scoped as one.

Much of the missing state is already arriving and being thrown away. The codec
decodes twelve telemetry families (`internal/codec/message.go`), but
`projectTelemetry` (`frontend/src/logic/sample.ts`) projects only eight into
`TelemetrySample`. Four reach the browser fully decoded and lose their payload
to the `default` branch, which keeps nothing but `sourceMessage` and
`receivedAtMs` for freshness:

| Family | ID | Carries | Shown |
|---|---|---|---|
| `STATUSTEXT` | 253 | severity, text — the log | nothing |
| `NAV_CONTROLLER_OUTPUT` | 62 | nav/target bearing, wp distance, alt/aspd/xtrack error | nothing |
| `RADIO_STATUS` | 109 | rssi, remrssi, noise, txbuf, errors | nothing |
| `HOME_POSITION` | 242 | home lat/lon/alt | nothing |

Fields inside projected families are also unrendered: battery cell voltages and
current, EKF variances, GPS `eph`/`epv`, and airspeed all sit in
`TelemetrySample` with no readout.

So a log panel and a materially denser display are reachable now, without new
backend work. What is genuinely absent from the backend should be established
by comparison against the reference rather than assumed.

## Outcome

The display carries the state density, layout, and message log of the
flight-hud-trajectory reference, built on the current backend, with anything the
backend cannot yet supply named as its own follow-up work.

## Scope

- In: the comparison against the reference, layout and density rework, and
  where each surface sits on screen.
- Out: projecting and rendering the discarded families (T-015), the
  `STATUSTEXT` log itself (T-016), the mission panel's restyle (T-014, which
  this may absorb or supersede), map viewport behaviour (T-012), mission
  mutation, and new MAVLink families until the comparison shows one is needed.

## Acceptance criteria

- [ ] The reference is described in the repository — screenshots or a written
      inventory of its panels, layout, and state — so parity is falsifiable
      rather than remembered.
- [ ] A gap list separates what the backend already supplies from what it does
      not, with a card for each genuine backend gap.
- [ ] Layout and density are judged against the reference in a browser, not
      against tests.
- [ ] The surfaces T-015 and T-016 deliver are placed deliberately rather than
      appended to the bottom of the display, which is how the mission panel
      arrived.
- [ ] Stale and absent data keep their existing posture: nothing new drives a
      number or instrument once stale.

## Verification

```sh
cd frontend && pnpm dev   # /?source=mock, compared against the reference
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run
```

## Open questions

- The reference artifact itself. It is not in this repository and cannot be
  recovered from git; it has to be supplied or described before parity means
  anything. This is what keeps the card in `backlog`.
- Whether the log is a panel in the display or a separate surface.
- Raw message inspection versus vehicle status messages: the candidate local
  reference has a general MAVLink stream log; T-016 covers only STATUSTEXT.
  STATUSTEXT is already recorded generically, as documented in T-016.
- How much of the density is layout and how much is genuinely missing data.
  Not answerable until the reference is available.

## Notes

2026-09-08 source discovery: see the provisional
[reference inventory](../../reference/ui-reference-review.md). The local
`flight-path-hud/apps/gcs` contains an inspectable candidate; confirmation of
the operator's reference name is pending. T-017 separates the first workspace
shell from this larger parity outcome so layout need not wait for T-015/T-016.

Raised 2026-09-08 after the operator reviewed the mission UI and judged the
display as a whole — not only the mission panel — well short of the benchmark.

The reachable-now work was split out on the same day so it is not blocked behind
the reference artifact: T-015 for the families that are decoded but discarded,
T-016 for the message log. What remains here is the part that genuinely needs
the reference — layout, density, and placement.

An earlier read of this feedback treated it as a mission-panel styling problem.
That was wrong and is recorded here so the card is not re-scoped down to a
restyle: the gap is layout, density, and a missing log, and the mission panel is
one symptom rather than the subject.

Current state is understood to be early rather than final. See
[[yalb-ui-emulate-flight-hud]] in operator memory for the design vocabulary the
rebuild should not diverge from.
