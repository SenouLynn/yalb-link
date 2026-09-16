---
id: T-018
title: Adapt figure-eight and traveling S-turn patterns for SITL validation
status: done
priority: 0
owner: unassigned
depends_on: T-019
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

- [x] Before building the compound patterns, demonstrate one simple baseline
      scenario (stationary, straight flight and a single turn as applicable)
      with setup → autopilot response → received MAVLink → backend state → UI
      evidence, plus controlled interruption and recovery.
- [x] Choose and document the first vehicle model, firmware, scenario driver,
      initial conditions, route, speed, altitude, duration and reset procedure.
- [x] The figure-eight completes two successive circuits with both turn
      directions and a crossing, within explicit position/altitude tolerances
      established for the chosen vehicle before acceptance.
- [x] The snake makes at least four alternating turns while progressing along
      a chosen direction; define its travel envelope and finite end behavior.
- [x] Telemetry comes from the autopilot responding to scenario inputs; no
      injected attitude or position replaces the simulated aircraft response.
- [x] Verify map track, heading/track distinction, prediction through turn
      reversals, altitude datum, and follow/manual-pan behavior in the browser.
- [x] Map each pattern to explicit validation claims: figure-eight crossings,
      turn reversals and bank transitions; snake sustained displacement, course
      changes, follow/manual camera behavior and bounded track history. Define
      expected behavior and applicable tolerances/deadlines before acceptance.
- [x] Compare simulator/autopilot logs and captured MAVLink with application
      output using aligned timestamps and independently checked interpretation.
      Distinguish commanded, observed and predicted paths in the evidence.
- [x] Measure five-second prediction error against actual later positions for
      the straight-flight baseline and turns; state assumptions, acceptance
      bounds and observed limitations instead of asserting visual accuracy.
- [x] Capture and replay a scenario recording; document the behavior checked
      and any differences from the live run.
- [x] Repeat a scenario with a controlled link interruption and verify stale
      posture and recovery; distinguish harness-injected faults from SITL inputs.
- [x] Record repeatable launch/reset procedures, actual results and limitations
      in the development runbook. Do not claim Plane and Copter parity from one.

## Verification

```sh
python3 -m unittest discover -s scripts -p 'test_sitl_motion.py'
python3 -m py_compile scripts/sitl_motion.py scripts/validation/analyze-motion.py
node --check scripts/validation/observe-motion.mjs
./scripts/kanban check
```

Execute the three live flights, independent analyzer/DataFlash comparisons and
browser/replay procedures in [the T-018 runbook](../../runbooks/validation/t018.md).
Fresh output is generated per run; historical captures are optional.

## Execution contract (2026-09-16)

Use the completed T-019 Copter 4.7.0 baseline and its external Guided driver.
Fly a sampled figure-eight (north = 40 sin(t), east = 20 sin(2t), 16
segments per circuit, two circuits) and a traveling snake (north = 15 i,
east = 20 sin(pi i / 2), i = 0..10). Requested speed is 5 m/s at
20 m relative altitude. Each waypoint must be reached within 3 m horizontally
and 2 m vertically in 90 s; land/disarm within 90 s. These are waypoint
patterns, not claims of analytic continuous motion. Run each from a recreated
local simulator; repeat figure-eight with an eight-second position interruption.
Compare five-second predictions with independent later TCP positions (turn
p95 <=30 m, alignment <=150 ms); retain T-019 straight-baseline evidence.
Use real browser capture, recording replay and DataFlash comparisons.

Verification: `python3 -m unittest discover -s scripts -p "test_sitl_motion.py"`,
scenario commands and gates in `docs/runbooks/validation/t018.md`, and
`./scripts/kanban check`. No hardware or operator choice is required.

## Open questions

None blocking for the first Copter waypoint-pattern pair. Smooth continuous
curves, wind and other vehicle models are outside this acceptance.

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


Completed 2026-09-16 by codex-motion. T-019 supplied the independently accepted
baseline. Three live Copter flights (recordings 9, 10, 11) reached all targets
and landed within their bounds. Figure-eight/snake/repeated figure-eight
five-second prediction p95 was 20.85/24.53/20.68 m, within the predeclared
30 m maneuver limit. The repeated run measured an 8.203 s position gap,
trajectory removal after 5.047 s and recovery 0.029 s after restoration.
Both camera drag/Follow checks passed; track stayed capped at 500. Independent
UDP/backend and DataFlash comparisons, rendered heading checks and both
figure-eight replays passed with zero browser exceptions. Nine focused tests,
script syntax/compilation and task-board checks passed.

These are finite polygonal Guided routes with corner slowing, not exact analytic
mock trajectories. The runbook records tolerances, actual results, replay cadence
differences and limitations. No Plane or hardware equivalence is claimed. The
ordinary simulator feed was restored; optional captures and Python caches are
ignored. T-014’s existing ownership/status was left untouched.

Cleanup 2026-09-16 at operator request: removed all T-018 temporary captures
and backend recordings 9, 10 and 11 after acceptance. Durable results and
reusable procedures remain in the runbook.
