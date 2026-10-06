---
id: T-050
title: Launch the hardware observer on a fresh machine
status: ready
priority: 1
owner: unassigned
depends_on: T-045
---

## Motivation and evidence

The documented Compose launch starts a development topology with SITL, not the accepted hardware observer journey.

Required by [ADR 0006](../../adr/0006-operator-connection-readiness.md).

## Outcome

A repeatable install and local launch procedure reaches the observer UI without a simulated aircraft or manual runtime configuration edits.

## Scope

Declare supported host/prerequisites, choose and implement a bounded launch/distribution path, local asset delivery, settings location and useful startup failure reporting. Offline behavior is separately verified in T-030.

## Acceptance criteria

- [ ] On a clean declared host environment, documented installation and launch reaches the local observer UI without SITL.
- [ ] Ordinary launch and connection setup do not require editing environment/configuration files; prerequisites and device-access requirements are explicit.
- [ ] The launch path reports backend/startup failures and handles occupied local service ports according to a documented policy.
- [ ] After installation, startup uses local assets and requires no internet; verify the resulting launch path against T-030 offline scenarios.
- [ ] Record actual fresh-machine verification and platform limitations without implying support for untested operating systems.

## Verification

Select the package/launch approach in T-045, then pin build and clean-environment manual commands. Repeat offline startup with empty browser cache and no SITL; run ./scripts/kanban check.

## Open questions

Initial OS, packaging and local server lifecycle remain open. Installation downloads may require internet; installed observer operation must not.

## Notes

Shaped 2026-10-06 per the [bench readiness plan](../bench-readiness-plan.md):
macOS-first, from a source checkout, no installer, no Windows. T-045 is done,
so the launch approach is chosen: the Go backend serves the built UI
(`GCS_UI_DIR`, `cmd/gcs/ui.go`) and `scripts/observer` builds, applies observer
defaults, checks the port, opens the browser and stops cleanly.
Procedure: [observer-launch.md](../runbooks/observer-launch.md).

Implemented and checked on the developer machine only (see that runbook's
evidence section): launch without SITL, local UI and API on one port,
occupied-port refusal, startup-failure reporting, clean shutdown, Go tests for
the static handler. **No acceptance box is ticked**: all of them name a clean
declared host or offline behavior, which is unverified. Remaining: fresh-machine
run, empty-cache offline start (with T-030), serial-ownership release checked
with a real controller, Bazel build of `cmd/gcs` (not runnable here).
