---
id: T-052
title: Prevent backward track samples from inflating flown distance
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

[T-019](../../runbooks/validation/t019.md) run 3 independently measured a
200.75 m route from TCP GLOBAL_POSITION_INT while replay showed 256 m Flown.
During steady northward motion the rendered track repeatedly stepped roughly
0.7 m backward, then forward. Exact UDP/backend position decoding passed.
Confirmed in fleet/state.ts: applyTelemetry passes each event partial directly
to accumulateGeoTrack. GPS events therefore bypass the merged sample’s global
position preference and add older sensor locations between global updates.

## Outcome

Track history and accumulated distance follow a coherent, fresh position stream
without periodic backward samples from older observations.

## Scope

Position selection, track insertion and distance accumulation; preserve GPS
fallback when global position is absent. No trajectory-model redesign.

## Acceptance criteria

- [x] Identify the actual event sequence causing the backward track insertion.
- [x] Regression covers interleaved GPS/global positions, out-of-order data,
  global-position loss/fallback/recovery and independent vehicle histories.
- [x] Repeat the T-019 baseline and compare visible track and Flown distance
  with independently decoded positions under a tolerance chosen before execution.

## Verification

```sh
cd frontend && pnpm test && pnpm lint && pnpm typecheck
./scripts/kanban check
```

Repeat the T-019 driver with a fresh backend and browser and output under
`docs/temp/evidence/T-052/RUN/`. Compare live and replay Flown at the end of the
route with independently decoded GLOBAL_POSITION_INT horizontal path length:
absolute difference <=5 m, fixed before execution. During steady north flight,
no backward track insertion larger than 0.2 m unless present in the independent
global-position stream. Keep the T-019 position/prediction/fault checks passing.
Regress the captured interleaving without requiring temporary evidence files.

## Open questions

No operator decision needed. Track source policy: valid global position wins
while fresh under the existing 5-second telemetry TTL; GPS can supply new fixes
when global is absent, invalid or stale. Use backend observation timestamps
(receipt fallback only for unstamped events), reject non-increasing position
observations and do not append stale retained bootstrap snapshots. No comparison
of GPS epoch time with global boot time. Hardware reboot/clock-epoch detection
is outside this change; a stream reset already clears accumulated history.
Quantify retained state across simulator resets separately using a fresh reducer
and a retained-state case; do not silently redefine session distance.

## Notes

Discovered during baseline acceptance on 2026-09-15; not implemented in T-019.
Optional raw/browser evidence is under docs/temp/evidence/T-019/2026-09-15-r3/.

Completed 2026-09-15. Fixed in fleet/state.ts's applyTelemetry: a fresh
GLOBAL_POSITION_INT is preferred over GPS_RAW_INT while within the telemetry
TTL, non-increasing per-family observation timestamps are rejected, and a
same-timestamp GPS fallback point is replaced (not double-appended) once
global position lands at that instant. Regression added in
fleet/positionTrack.test.ts covering interleaved sources, the exact TTL
boundary, invalid coordinates on either family, duplicate/out-of-order
observations, per-vehicle independence and reset/retained-session distance.

Repeat of the T-019 route (see [the T-052 runbook](../../runbooks/validation/t052.md))
against a fresh simulator: independent raw route 200.745 m; live and replay
Flown both read 200.0 m; zero backward track steps in the steady northward
corridor for raw, live or replay. All predeclared analyze-track.py gates
passed. The first attempt's live browser capture never opened the vehicle
panel because scripts/validation/observe-motion.mjs clicked "Open selected
vehicle" once, unconditionally, before the fleet bootstrap had reported any
vehicle; fixed to poll for the button becoming enabled, matching how the
replay branch already polls for "Play". That incomplete run is kept at
docs/temp/evidence/T-052/2026-09-15-r1-incomplete/ for the failure mode; the
accepted run is docs/temp/evidence/T-052/2026-09-15-r2/.

pnpm test/lint/typecheck pass; the 4 pre-existing frontend failures unrelated
to this change (battery format, vehicle-selector text, App.test.tsx) are
untouched. ./scripts/kanban check passes. Task-started containers were
restored to the ordinary (non-relay) simulator link; the recordings volume
was retained.
