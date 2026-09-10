# Operator usage and end-to-end acceptance — WIP

This is the operator's intended build trajectory, captured 2026-09-10, not a
claim of implemented capability or flight readiness. Maintain scenarios here
as intent; executable work and ownership live in the [task board](README.md).
An ADR should follow when an experiment settles a costly application boundary.

## Real-world starting point

The operator reports ArduPilot **Plane 4.6.x, not 4.7**, on a **MicoAir H743 v1**,
configured as a tricopter QuadPlane. Exact firmware patch/build, board identity,
frame configuration and parameter snapshot remain to be captured from the
actual controller. Do not substitute a generic Copter or conventional Plane
acceptance result for this configuration.

Two physical paths matter:

- Bench: laptop USB-C directly to the controller, with no flight battery, for
  parameter inspection and eventually deliberate configuration edits such as
  channel mapping. Which peripherals and readings work on USB power is unverified.
- Field: powered aircraft → MicoAir LR900-F / LR900-P 900 MHz telemetry pair →
  ground unit USB-C → laptop. Air unit is connected to the controller. Exact
  serial settings, radio configuration, throughput and reconnect behavior are
  unverified; the model names alone do not establish compatibility or capacity.

Initially the RC pilot controls flight and MissionPlanner prepares missions.
The application observes the vehicle and the onboard mission. MissionPlanner
remains the fallback for configuration and waypoint preparation; replacing it
is not a prerequisite for field telemetry acceptance. Plan tool handoff explicitly
rather than assuming both programs can own the same USB device simultaneously.

## Current evidence and the change in direction

The [README](../../README.md) describes working UDP ingestion, telemetry display,
recording/replay and read-only mission download. The executable entry point
currently opens a UDP endpoint ([main.go](../../cmd/gcs/main.go)); direct USB
transport is not an accepted product workflow. Parameter and mission write
encoders in [encode.go](../../internal/codec/encode.go) are protocol primitives,
not completed configuration or mission-editing applications.

[T-008](cards/T-008-live-mission-inspection-acceptance.md) supplies Copter/Plane
SITL mission evidence. [T-015](../runbooks/validation/t015.md) supplies display
evidence. Neither proves this controller, Plane 4.6.x QuadPlane, USB-only power,
radio performance or operation with the internet disconnected.
[T-019](cards/T-019-sitl-motion-baseline.md) explicitly targets Copter 4.7.0 (+);
it remains useful regression work but cannot be the hardware compatibility gate.

The existing opt-in arm/disarm path and [ADR 0004](../adr/0004-operator-command-transactions.md)
remain current behavior. Their existence does not authorize expanding commands
or establish a navigation transaction design. Field observation acceptance uses
operator commands disabled.

## Capability order and isolation requirements

| Stage | Intended operator outcome | Gate and boundary |
|---|---|---|
| 1. Read foundation | Connect USB, identify the controller, inspect state and eventually parameters | Establish the exact target and transport; parameter reads are their own slice |
| 2. Field observer | Fly using RC and a MissionPlanner mission; trust radio-fed state without internet | Applicable target SITL checks, powered ground radio checks, then field evidence |
| 3. Bench configuration | Preview and deliberately apply a small configuration change over USB | Separate write policy, service/API boundary, tests and work cards; confirmed readback and uncertainty handling |
| 4. Mission preparation | Create waypoints offline and deliberately upload a mission while grounded | Field read paths accepted first; editor and upload are separate outcomes |
| 5. Flight navigation | Request lift-off, a waypoint, next waypoint or return home from the laptop | Separate command semantics and acceptance per action; upload success is not execution success |
| 6. PID apparatus | Pilot flies while a second person adjusts a bounded tuning value | Depends on validated read and write foundations; separate planning, policy and UI; ground experiment before live trials |

USB inspection can advance alongside field read work. Bench writes are a later
optional branch and must not hold up the observer. Mission preparation also
remains optional while MissionPlanner meets that need. PID work follows the
preceding foundations rather than becoming a convenient generic parameter UI.

“Read-only” means no operator mutation of configuration, mission, arming or
flight intent. It does **not** mean zero outbound MAVLink: heartbeat, telemetry
rate requests and mission/parameter read requests may be required. Enumerate
and test allowed acquisition traffic; keep rate configuration distinct from
vehicle configuration. Hiding buttons is insufficient isolation.

Proposed implementation direction: share transport, decoding and read models,
but require separate explicit capability gates and narrow mutation services for
Bench, mission upload, navigation and tuning. A separate Bench executable/app
is a candidate, not an accepted decision. Before its first write implementation,
settle the process/deployment boundary and startup defaults in an ADR. The
existing command switch must not silently enable these future capabilities.

Each write slice must define target identity, preconditions, exact proposed
change, confirmation, readback/observed outcome, timeouts, interrupted delivery,
restart behavior and audit evidence. Never report a timeout as “not applied,”
blindly retry an uncertain change, or imply a multi-parameter change is atomic.
Recovery and rollback require verification; they are not assumed guarantees.

## Intended end-to-end scenario register

**All scenarios below are planned and unexecuted for the actual aircraft.**
Prior automated/SITL passes remain scoped to their recorded environments.

| ID | Setup and operator action | Observable acceptance | Failure/recovery coverage |
|---|---|---|---|
| E2E-01 USB observation | Battery disconnected; attach controller USB; select vehicle | Correct identity/version, fresh available telemetry; unavailable power/GPS/home clearly absent; no mutation traffic | Unplug/replug, device renumbering, backend/browser restart; old state never becomes fresh on reconnect |
| E2E-02 Parameter inspection | On identified USB controller, request and search parameters including channel mapping | Names, types and values agree with independently captured controller/MissionPlanner data; completeness and age explicit | Partial transfer, duplicates, timeout and reconnect; no PARAM_SET; no mixing vehicle snapshots |
| E2E-03 Offline powered ground | Start local stack with internet disconnected and empty browser cache; connect radio pair | UI/assets load locally; vehicle, readings and onboard mission work; missing imagery is explicit while position/track/mission remain usable | Radio interruption, USB reconnect, constrained throughput, browser reload; recording/replay works offline |
| E2E-04 RC field observation | RC pilot flies a preloaded mission/flight plan; application commands disabled | Compare attitude, mode/armed state, position, altitude datum, speed/climb, battery, GPS/EKF, home/guidance, mission progress and link status with independent autopilot evidence | Delayed/absent families show uncertainty; post-flight link-loss replay; do not intentionally introduce hazardous airborne faults |
| E2E-05 Bench change | Grounded controller; inspect baseline, preview one supported configuration edit, apply | Only intended target/value changes; readback distinguishes applied, rejected and unknown; reboot-required/persistence semantics recorded | Disconnect during apply, rejection, stale baseline, restart; verify recovery and retained baseline |
| E2E-06 Offline mission preparation | Grounded vehicle; create/edit waypoint draft without internet, preview and upload deliberately | Coordinate frame, altitude datum, item order and supported commands explicit; onboard download matches intended mission; upload does not start flight | Partial upload, timeout, reconnect and vehicle switch; preserve draft and report uncertain onboard contents |
| E2E-07 Navigation commands | First target SITL, later separately approved field procedure; request one action | Lift-off, go-to, next-waypoint and RTH each have mode-specific prerequisites and observed completion criteria; ACK alone is insufficient | Rejected/late ACK, link loss, uncertain outcome and RC intervention; no automatic retry or implicit mode transition |
| E2E-08 Tuning experiment | First ground/SITL, later pilot plus tuning operator; change one eligible PID parameter | Exact firmware-specific eligibility and bounds, baseline, proposed/applied/readback values and aligned flight response recorded | Reject unsupported/stale/out-of-range edits; interrupted delivery stays uncertain; explicit verified recovery procedure |

Before running a scenario, fill in this record; do not choose tolerances after
seeing the result:

- Scenario ID, date, tester, application revision, firmware version/build, board,
  frame and baseline parameter/mission identifiers.
- Wiring/power/transport, radio settings, host environment, startup procedure,
  internet/cache state and operator capability gates.
- Inputs/actions, expected values with units/frames/datums, per-reading freshness
  limits, recovery deadlines and comparison tolerances.
- Independent truth source (autopilot logs or separately captured MAVLink and
  known setup), clock alignment, backend/SSE and visible UI observations.
- Fault injection method and environment; distinguish harness faults from
  real radio behavior. Use SITL/ground tests for disruptive failures.
- Actual result: not run / pass / fail / blocked; discrepancies, durable runbook
  link, temporary capture location and limits of the claim.

Store raw captures under `docs/temp/evidence/<task-id>/<run-id>/` according to
[artifact policy](../temp/README.md). Retain conclusions and procedures in a
runbook. A passed replay confirms reproducibility, not independent correctness.

## Next scoped work

Prioritize [T-023](cards/T-023-rate-requests-after-source-change.md) reconnect
recovery alongside [T-027](cards/T-027-target-profile.md), the target profile.
Then [T-028](cards/T-028-usb-observer.md) demonstrates USB observation and
[T-029](cards/T-029-parameter-inspection.md) adds parameter inspection.
[T-030](cards/T-030-offline-observer.md) establishes offline behavior;
[T-031](cards/T-031-radio-ground-acceptance.md) measures the real radio ground
path before [T-032](cards/T-032-rc-field-acceptance.md) field observation.

Keep [T-016](cards/T-016-statustext-log.md) status messages relevant to operator
inspection; determine required missing state during the target acceptance plan.
HUD parity, mission styling and compound motion scenarios remain useful backlog
work, but they no longer define the next hardware-use milestone. No existing
card's completed evidence or ownership is rewritten by this plan.

Later write design is deliberately separate:
[T-033](cards/T-033-bench-write-boundary.md) Bench boundary,
[T-034](cards/T-034-offline-mission-draft.md) offline drafts,
[T-035](cards/T-035-mission-upload-contract.md) upload contract,
[T-036](cards/T-036-navigation-contracts.md) flight navigation contracts and
[T-037](cards/T-037-pid-feasibility.md) PID feasibility. These are design/discovery
outcomes; they do not authorize bundling implementation or live flight trials.

## Unsettled inputs

Capture the exact 4.6.x firmware and frame configuration, laptop OS/device access,
radio serial settings, and which USB-only peripherals are present in T-027.
Decide whether offline geographic context requires preloaded imagery or whether
an imagery-free map suffices for initial observation; do not select a tile
storage architecture merely to pass an offline startup test.

Before writes, resolve Bench packaging, firmware-specific parameter metadata,
persistence/reboot behavior and supported mission commands. Before navigation,
define what “lift-off,” “go-to,” “next waypoint” and RTH mean in the active
QuadPlane mode, including transitions and operator handoff. Before PID work,
verify which exact parameters can be changed while armed and when they take
effect; live tuning feasibility is a question, not a promised capability.
