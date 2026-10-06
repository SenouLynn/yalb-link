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
