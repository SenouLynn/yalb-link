---
id: T-050
title: Launch the hardware observer on a fresh machine
status: backlog
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

Planned, not implemented or accepted. Refine verification commands and scenario
bounds before promoting implementation work to ready.
