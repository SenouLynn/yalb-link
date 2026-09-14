# ADR 0006: Establish operator connection readiness before expanding capabilities

Date: 2026-09-14

Status: Accepted — milestone and behavioral boundaries. Implementation choices
below remain open; acceptance of this decision is not hardware validation.

## Context

The operator considers the Vehicle UI sufficient to move beyond visual work.
The long-term capability order is already captured in the
[operator usage plan](../tasks/operator-usage-plan.md): read foundation, field
observation, then separately scoped configuration, mission preparation,
navigation and tuning. The immediate problem is the missing path from a fresh
installation to dependable observation of the actual aircraft.

The current executable opens a MAVLink UDP listener. The documented Compose
launch is a development topology with SITL. Serial device discovery, selection,
saved connection settings and an accepted hardware startup procedure are absent.
Browser stream connectivity, vehicle liveness and received radio measurements
exist, but do not constitute a diagnosis of the complete connection path.
T-023 records a source-change recovery defect. Actual USB and radio acceptance
remain unexecuted.

These are intermediary requirements and blockers to practical use. A polished
vehicle display and successful simulation do not demonstrate that an operator
can install, connect, identify, disconnect and recover in the intended setting.

## Decision

Make **install, select, save, connect and recover** the next coherent operator
milestone. Establish this foundation before expanding operator capabilities.
Execute it as independently verifiable task slices, preserving the long-term
plan and existing evidence. Parameter reads follow the connection foundation;
configuration writes, uploads and flight commands retain their separate gates.

The acceptance contract is:

> On a fresh machine, the operator can launch the observer, recognize and save
> a connection, observe the aircraft, hand the port to MissionPlanner and back,
> and recover from either end being unplugged or powered off without editing
> configuration files or restarting services as a recovery procedure.

### Operator journeys

1. **First launch:** provide a documented local observer launch without a
   simulator. The application and connection setup must work without internet.
   Missing imagery must be explicit and must not prevent telemetry observation.
   Opening the application before or after attaching hardware must both work.
2. **Bench observation:** select the USB controller and necessary connection
   settings, identify the reporting controller, and inspect available state
   without a flight battery. Distinguish absent readings from reported faults;
   USB-only peripheral availability requires evidence from the actual setup.
3. **Field observation:** select the ground radio, which may be attached before
   the aircraft is powered. An open local port waiting for aircraft telemetry
   is an ordinary state. Identify reporting vehicles when they arrive; RC
   remains in control and MissionPlanner prepares missions.
4. **Tool handoff:** explicitly disconnect to release the device for
   MissionPlanner. An intentional disconnect stops automatic reconnection.
   Reconnecting after handoff must not require restarting the application.
5. **Interruption and return:** handle both ground-device removal and loss of
   aircraft traffic while the ground device remains attached. Resume an active
   connection when its identifiable device returns. Recover acquisition after
   both brief interruptions and declared vehicle loss.

### Connection ownership and persistence

Connection management belongs to the application and must be reachable from an
empty fleet and from vehicle detail. Keep a compact connection summary visible
during observation; detailed selection and diagnosis belong in a dedicated
surface. Preserve fleet-first navigation: a connection is an acquisition path,
not a vehicle identity, and can carry multiple vehicles.

Persist recognizable local connection profiles, such as Bench controller and
Field radio, with the required device and communication settings across browser
and application restarts. Profiles do not grant command capabilities. Prefer
stable device identity when available; do not silently substitute another device
because it occupies a saved COM-port name. Ambiguous identity requires selection.
Remembering settings and deciding whether to connect on a new application launch
are distinct behaviors; the latter remains to be specified.

### Status must describe the evidence

Distinguish application/backend availability, local device presence and port
access, receipt of valid MAVLink, identified vehicle liveness, and freshness of
individual readings. Browser connectivity must not imply aircraft connectivity.
Port access must not imply a working air link or flight readiness.

Expose useful states such as saved device missing, port open awaiting telemetry,
port access failure, vehicle reporting and vehicle traffic interrupted. Show
actionable errors without inventing causes: silence alone cannot establish a
wrong device, incorrect baud rate, powered-off aircraft or failed radio path.
Name another application's ownership of a port only when established.

### Recovery preserves context without inventing freshness

Keep the selected vehicle and workspace through interruption. Preserve useful
history, including last known position, with explicit age and stale treatment.
An open ground port and absent aircraft traffic must remain distinguishable.
Do not infer that the aircraft was powered off from telemetry silence.

When traffic returns, resume acquisition automatically for the active connection
and make readings current only as fresh observations arrive. Retained mission
and other snapshots do not become newly verified on reconnection. A different
reporting vehicle must not silently inherit the previous vehicle's state.
Recovery must cover interruptions shorter than the vehicle-loss threshold,
including acquisition that needs renewed rate requests.

## Consequences and work sequencing

- T-023 reconnect recovery is necessary but does not complete this milestone.
- T-027 establishes the exact Plane 4.6.x QuadPlane, host and device profile.
- T-028 must be shaped to include the connection workflow and USB acceptance,
  with separate implementation cards for independently mergeable outcomes.
- T-030 covers offline startup; a repeatable fresh-machine hardware launch is
  also required, beyond merely demonstrating the existing development launch.
- T-031 verifies the real radio ground path, including airborne equipment power
  cycles and ground USB removal. T-032 follows applicable ground acceptance.
- T-029 parameter inspection builds on this foundation. Existing display,
  simulation and UI evidence retain their original scope.

Before implementation, shape cards for launch/distribution, serial discovery
and access, saved profiles, connection status and control, and recovery. This
ADR establishes their requirements, not a claim that those cards or features
already exist. Track executable work on the file-backed board rather than
maintaining a second status index here.

This decision rejects treating a COM-port picker alone, or the T-023 repair
alone, as sufficient operator readiness. An external serial-to-UDP adapter may
be useful experimentally, but a manually configured bridge does not by itself
satisfy the accepted installation and connection journey.

## Open implementation choices and blockers

- Supported initial operating system, device drivers/access and exact hardware
  identity, firmware, baud and radio settings require target evidence.
- Native serial ingress versus a managed adapter, packaging/launcher, profile
  storage and migration, and device identity fallback need scoped decisions.
- Startup connection policy, retry timing, interruption thresholds, reboot
  detection and snapshot revalidation need explicit behavioral contracts.
- Radio diagnostics available from the actual LR900 pair remain unverified;
  do not assume a particular radio-status protocol or diagnosis capability.
- Offline imagery expectations remain open; this ADR selects no tile storage
  architecture.

These unknowns constrain implementation and acceptance, but do not block
documenting the requirements or undertaking independent software work. Mark a
task blocked only when its progress actually requires an external decision.

## Verification point

Accept the milestone through repeatable operator journeys on the declared target
environment, with expected timing and freshness bounds chosen before execution:

- Fresh installation and offline launch with empty browser cache; hardware
  attached before and after startup; no simulated vehicle mistaken for hardware.
- Initial device selection, persisted settings, application/browser restart,
  changed port name, absent saved device and ambiguous replacement.
- Unrelated or silent device, failed/busy port access, and MissionPlanner
  release/reacquire with no automatic retry after intentional disconnect.
- Battery-free controller observation and powered ground radio observation.
- Short and long aircraft-traffic interruptions, airborne equipment power
  cycles, ground USB unplug/replug and backend/browser interruption.
- Preserved vehicle context, explicit stale readings, correct identity and fresh
  acquisition after recovery, with retained snapshots still honestly labeled.

Use applicable target SITL and automated fault tests first, then hardware for
the gaps simulation cannot establish. Follow the operator plan's evidence
record and task board's verification conventions. Do not introduce disruptive
faults during flight to satisfy ground recovery checks. Record outcomes and
remaining limitations separately from planned scenarios.

At adoption, these journeys remain **planned and unexecuted on the actual
aircraft**. This ADR records an important transition in priorities and the
requirements for success, not a declaration of real-world readiness.
