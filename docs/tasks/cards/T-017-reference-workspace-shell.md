---
id: T-017
title: Establish the reference-driven vehicle workspace before adding panels
status: backlog
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator requested a UI-led phase using the original interface as a strong
signal for layout, modularity, hierarchy and features while retaining this
repository's backend. [Source review](../../reference/ui-reference-review.md)
identifies a candidate reference and concrete composition differences.
T-013 currently waits for T-015/T-016; a usable workspace shell need not wait
for every telemetry or log feature.

The existing telemetry, recording/replay, guarded command and mission paths
provide enough infrastructure to validate through operator workflows. T-009
and T-010 document lifecycle and live-surface defects discovered after the
initial mission implementation. Use this UI phase to expose further integration
defects; a fixture-only visual redesign would leave that feedback loop open.

## Outcome

Existing working capabilities occupy a coherent vehicle workspace informed by
the confirmed reference, with deliberate space for subsequent inspection work.

## Scope

- In: shell, compact context sidebar, responsive map/instrument/mission panes,
  panel visibility controls and placement of existing replay/arm controls.
- Out: new transport, command endpoints, video, fleet overview, advanced HUD
  instruments, new telemetry projection and message accumulation.

## Acceptance criteria

- [ ] Confirm the reference location and primary view before promotion to ready.
- [ ] At 1440×900, map and instruments are visible together; the mission list
      scrolls within its pane and does not push the map below the page.
- [ ] At 768×1024, panes remain reachable without horizontal document overflow.
- [ ] Pane visibility changes do not restart the event source, reset selected
      vehicle state, or lose an in-flight mission request.
- [ ] Mock, live, replay, stale/absent readings, selected-vehicle isolation and
      guarded arm transaction behavior retain their existing semantics.
- [ ] Capture browser comparisons with the reference at both sizes and exercise
      download, pane toggles, vehicle switch, replay and disconnected posture.
- [ ] Record results for applicable workflows in mock, live SITL and recording
      replay, including disconnect/reconnect, stale/absent data, vehicle switches
      during requests, and explicit request failures. State unsupported paths
      and unexecuted checks rather than counting fixture success as live evidence.
- [ ] Fix discovered integration defects in scope or create follow-up cards with
      reproduction steps; unresolved defects affecting this outcome block completion.

## Verification

Use SITL through the normal backend as the primary development feed, following
the [scenario conventions](../README.md#sitl-first-validation). Synthetic
browser fixtures support iteration; they do not replace end-to-end acceptance.
Real flight-controller ingestion is deferred until an identified validation
gap requires it after the relevant SITL scenarios pass.

Run frontend typecheck, lint and the existing Vitest suite. Build the frontend.
Then perform the browser checks above; test state ownership changes with focused
lifecycle tests. Record actual commands and evidence during implementation.

## Open questions

- Is the local `flight-path-hud/apps/gcs` the intended reference? An operator
  clarification is pending; the source inventory remains explicitly provisional.

## Notes

Planning slice only. T-013 remains the larger parity outcome. After confirming
the reference, implement this shell first, place T-015/T-016 inside it, and use
T-012/T-008 to exercise mission capability. Coordinate T-014 with the reference
palette instead of treating the current stylesheet as an immutable design.

Work in UI-led vertical slices: compose existing capabilities, exercise them
end to end, then add telemetry inspection and logs in their intended locations.
T-008 remains the owner of full Copter/Plane mission acceptance; it is part of
this phase, not a prerequisite for beginning workspace work. Keep backend
changes tied to demonstrated gaps such as log continuity or missing telemetry
delivery. Authentication, shared persistence and additional command surfaces
remain outside this slice under the existing local-operator scope.

Record executed procedures and observed results in the development runbook
when verification occurs. The checks above are planned acceptance, not evidence
that those scenarios have already passed.
