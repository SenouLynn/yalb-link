---
id: T-053
title: Prove live connection UI and attitude before USB bench recovery work
status: ready
priority: 0
owner: unassigned
depends_on: T-046, T-047, T-048
---

## Motivation and evidence

The [bench readiness plan](../bench-readiness-plan.md) targets USB-powered,
battery-free attitude observation. T-048 has real open/release API evidence but
its Notes explicitly exclude real REPORTING and ambiguous identity integration.
T-047 lacks a real-backend profile restart harness. Current T-048 work is
uncommitted and must be preserved.

## Outcome

A repeatable real-backend serial/SITL-to-browser baseline establishes which
startup, profile and attitude behaviors already work before T-049/T-050.

## Scope

- In: controlled PTY/target SITL integration, actual connection UI, profile
  reload/restart, attitude path, small regressions found in that path.
- Out: physical controller acceptance, automatic retry implementation, packaging,
  raw accelerometer display, radio and configuration writes.

## Acceptance criteria

- [ ] Current focused checks are run; any failures are named with reproduction
  and disposition, including the older failures mentioned by T-048.
- [ ] Actual browser controls complete empty fleet → connect → REPORTING →
  vehicle → fresh attitude → release against real backend MAVLink traffic.
- [ ] Independent decoded ATTITUDE matches displayed sign, axes and values
  within display rounding; stale attitude clears while heartbeat continues.
- [ ] Saved profile survives browser/backend restart; missing/ambiguous identity
  requires selection and explicit release remains released after restart.
- [ ] Timing expectations are declared before execution; actual timings and
  unsupported recovery cases are recorded without claiming T-049 complete.
- [ ] Outbound traffic stays within the observer allowlist, and evidence/runbook
  separates simulated serial success from untested physical USB behavior.

## Verification

```sh
go test -race ./internal/connection/... ./internal/bridge/... ./internal/stream/... ./cmd/gcs/...
cd frontend
pnpm test -- src/connections src/stream src/logic/attitude.test.ts src/ui/FlightDisplay.lifecycle.test.tsx
pnpm typecheck
pnpm build
cd ..
CGO_ENABLED=1 go build -o /tmp/yalb-bench-gcs ./cmd/gcs
python3 scripts/validation/serial-acquisition.py --binary /tmp/yalb-bench-gcs --check-silence
./scripts/kanban check
```

Follow the target SITL procedure in `docs/runbooks/validation/t046.md`. Extend
the controlled harness as necessary to keep a MAVLink feed available while
driving the actual UI, inject attitude-only silence and profile identity faults,
and capture aligned wire/backend/browser evidence. The standalone harness is
not a substitute for that browser pass.

## Open questions

None blocking this controlled baseline. Physical host/board/build and raw
accelerometer requirements are separate from proving the existing attitude path.

## Notes

Created 2026-10-06 during planning; verification above is unexecuted for this
card. No implementation has been claimed or performed by this planning pass.

### Execution 2026-10-06 (partial; card stays `ready`)

Rig: `scripts/validation/bench-rig.py` (synthetic aircraft on a PTY: 1 Hz
HEARTBEAT, 10 Hz ATTITUDE, steerable; real native backend; `vite dev` proxied
to it; headless Chrome) driven through `scripts/validation/cdp.mjs`. macOS
arm64, PTY only — **no USB, no physical controller, no target SITL run.**

Baseline checks: `go test -race` on the four packages, `serial-acquisition.py
--check-silence` (PASS; outbound limited to msgs 0/76/43/47) and `kanban check`
pass; `pnpm typecheck` and `pnpm build` pass. Frontend: exactly the four
pre-existing failures (`App.test.tsx` StrictMode clock test; three in
`FlightDisplay.test.tsx`: lost-vehicle selector, second-vehicle selector,
battery format). Unchanged, unrelated to this card; still open.

Defects found by the real-browser pass and fixed (each has a regression test):

1. `stream/live.ts` never subscribed to `acquisition` SSE events, so the
   connection summary stayed at the connect response (AWAITING TRAFFIC) while
   the backend reported REPORTING. Only a reload corrected it.
2. "Save & connect" connected by profile alone. A device with no USB
   serial/VID/PID cannot be identity-matched, so it returned 404 "device is not
   in the current inventory". It now passes the operator-selected device id.
3. Saved profiles with no live status (never connected, or listed after a
   failed connect) had no row, so there was nothing to reconnect from. They now
   render as "Saved" with Connect/Delete.

Observed, working: empty fleet loads with connection controls → add/save →
connect → REPORTING → vehicle 41:1 → ATTITUDE. Five poses
((-30,-15) (0,0) (45,25) (-10,30) (5,-5)) displayed exactly as transmitted,
sign and axes agreeing, UI update 42–83 ms after the pose change (includes up
to one 100 ms transmit period). Attitude-only silence with heartbeat
continuing: value held and aging to ~4 s, roll/pitch dashed at ~5 s, recovered
within 0.6 s of resuming. Release from the UI persisted across backend restart
(RELEASED, intent RELEASED). A CONNECTED profile without USB identity came back
as DEVICE_MISSING after restart (no automatic reacquisition: T-049), the UI
offered a device picker, and Select reached REPORTING.

Discrepancy to resolve (T-027 audit): attitude freshness uses the generic
`TELEMETRY_TTL_MS` (5 s, `ui/readings.ts`), not three message intervals
(~0.3 s at 10 Hz). Needs a decision before T-028: keep 5 s or add a per-message
TTL.

Other observations: `acquisition` events are emitted on essentially every
received frame (~10/s); on a Mac the device list includes Bluetooth/headphone/
debug ports alongside the controller; saved-profile Connect on an
identity-less device shows the backend text rather than prompting selection.

Not done (acceptance boxes remain open): independent wire-side decode check
(sent values were used as ground truth); ambiguous-identity inventory; saving
via actual USB metadata; target Plane 4.6.3 SITL browser pass; browser-reload
with an *identified* profile; recording replay; timing expectations were not
declared ahead of the run; outbound traffic was counted by message id only, not
checked per command id.

