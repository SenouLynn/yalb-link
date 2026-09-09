---
id: T-018
title: Adapt figure-eight and traveling S-turn patterns for SITL validation
status: backlog
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The operator found the original figure-eight and one-direction snake especially
useful for validation. The [recovered profiles](../../reference/ui-reference-review.md#recovered-motion-profiles)
are analytic/integrated mock kinematics transmitted as JSON UDP envelopes,
not ArduPilot SITL flights. Reuse their validation purpose within the
[SITL-first workflow](../README.md#sitl-first-validation).

## Outcome

A developer can start repeatable figure-eight and traveling S-turn scenarios
in SITL and observe simulator-generated flight telemetry through YALB's normal
binary MAVLink, backend, browser and recording paths.

## Scope

- In: a first vehicle-specific scenario pair, repeatable initialization and
  reset, external simulator setup tooling, and recorded/browser evidence.
- Out: YALB mission-write or mode-change endpoints, copying the old JSON
  transport, hardware ingestion, and exact reproduction of synthetic telemetry.

## Acceptance criteria

- [ ] Before building the compound patterns, demonstrate one simple baseline
      scenario (stationary, straight flight and a single turn as applicable)
      with setup → autopilot response → received MAVLink → backend state → UI
      evidence, plus controlled interruption and recovery.
- [ ] Choose and document the first vehicle model, firmware, scenario driver,
      initial conditions, route, speed, altitude, duration and reset procedure.
- [ ] The figure-eight completes two successive circuits with both turn
      directions and a crossing, within explicit position/altitude tolerances
      established for the chosen vehicle before acceptance.
- [ ] The snake makes at least four alternating turns while progressing along
      a chosen direction; define its travel envelope and finite end behavior.
- [ ] Telemetry comes from the autopilot responding to scenario inputs; no
      injected attitude or position replaces the simulated aircraft response.
- [ ] Verify map track, heading/track distinction, prediction through turn
      reversals, altitude datum, and follow/manual-pan behavior in the browser.
- [ ] Map each pattern to explicit validation claims: figure-eight crossings,
      turn reversals and bank transitions; snake sustained displacement, course
      changes, follow/manual camera behavior and bounded track history. Define
      expected behavior and applicable tolerances/deadlines before acceptance.
- [ ] Compare simulator/autopilot logs and captured MAVLink with application
      output using aligned timestamps and independently checked interpretation.
      Distinguish commanded, observed and predicted paths in the evidence.
- [ ] Measure five-second prediction error against actual later positions for
      the straight-flight baseline and turns; state assumptions, acceptance
      bounds and observed limitations instead of asserting visual accuracy.
- [ ] Capture and replay a scenario recording; document the behavior checked
      and any differences from the live run.
- [ ] Repeat a scenario with a controlled link interruption and verify stale
      posture and recovery; distinguish harness-injected faults from SITL inputs.
- [ ] Record repeatable launch/reset procedures, actual results and limitations
      in the development runbook. Do not claim Plane and Copter parity from one.

## Verification

During shaping, establish concrete launch and scenario commands and measurable
flight tolerances. During implementation, test route geometry and runner failure
handling where introduced, then execute the scenarios against SITL and the
browser and run the task-board check. Synthetic-profile success alone does not
satisfy this outcome.

## Open questions

- Which first vehicle and route scale yield useful continuous turns? The old
  profile dimensions and timing are reference values, not flight constraints.
- Use externally loaded waypoint missions or a bounded Guided scenario driver?
  Evaluate existing simulator setup tools before introducing another dependency.
- Define vehicle-appropriate tolerances and end/reset behavior before promoting
  to ready. Actual attitude, heading and track need not match analytic mocks.

## Notes

This complements T-017 and can supply motion evidence for UI work without
making a complete scenario library a prerequisite for starting that work.
Existing T-001/T-002 flight procedures remain available in the meantime.

The first deliverable is the baseline evidence chain, not a complete scenario
library. During shaping, split it into its own ready card if it is independently
mergeable. Reuse existing decoding and logic checks; preserve newly discovered
defects as focused regression tests. Report what each scenario proves, its
conditions and remaining gaps. Neither exact agreement with the original mock
nor replay agreement alone establishes end-to-end correctness.
