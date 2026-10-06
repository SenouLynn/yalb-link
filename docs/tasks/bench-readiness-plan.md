# USB bench readiness plan

Prepared 2026-10-06 from the current working tree. This is planned work, not
hardware acceptance. Task status remains in the individual cards.

## First outcome

Launch the local observer, attach an ArduPilot controller over a USB data
cable with no flight battery, select/connect it in the UI, and see fresh roll
and pitch follow gentle controller tilts. Unplugging must visibly age the
instruments; reconnecting must recover without restarting the application.
USB supplies controller power: an electrically unpowered controller cannot
send telemetry. Keep operator commands disabled throughout this milestone.

Initial host assumption: the current macOS development machine. The recorded
target is MicoAir H743 v1 / Plane 4.6.x tricopter QuadPlane; confirm board,
actual firmware build and host at the physical session rather than treating
the older profile as independently verified hardware evidence.

The horizon consumes ArduPilot's estimated `ATTITUDE` roll/pitch, not raw
accelerometer samples. This is the first motion-feedback acceptance target.
Raw acceleration X/Y/Z would require a separate slice: select the supported
IMU message and sensor instance, request/decode it, define units/body axes and
freshness, extend telemetry/recording, and add sensor readouts. Do not imply
that the present horizon is a raw accelerometer display.

## What exists and what remains unproved

| Area | Evidence in this checkout | Remaining gap |
|---|---|---|
| Native serial | T-046; inventory/open/release API; macOS/Linux PTY and target SITL evidence | Real controller enumeration, USB boot behavior and unplug timing |
| Saved profiles | T-047; persisted identity and connection intent | Real-backend restart/renumbering integration evidence |
| Connection UI | T-048; current uncommitted frontend work, fixtures and real API open/release evidence | UI with actual MAVLink REPORTING; ambiguous identity integration |
| Instruments | `DefaultRates` requests ATTITUDE at 10 Hz; attitude resolver and horizon exist | USB tilt → received packet → backend → visible horizon evidence |
| Recovery | Source-change rate requests exist; manager detects loss/silence | T-049 automatic reacquisition and short interruptions |
| Startup | Native backend and Vite can be run separately | T-050 repeatable hardware-only launch and startup error handling |

Preserve existing staged/unstaged T-048 changes. Its Notes report four older
frontend failures; reproduce and name current failures before deciding whether
they affect this milestone. Older narrative claims that acquisition is UDP-only
or that connection controls are still absent are superseded by the code and
T-046/T-048 evidence, not evidence of missing implementation.

## Execution order and gates

1. **Prove the current integration — T-053.** Run focused regression checks,
   use the existing serial PTY harness, then drive the actual connection UI
   against MAVLink-producing PTY/target SITL. Verify empty fleet → select →
   open → reporting → vehicle → moving attitude → release. Verify profile
   save, browser reload and backend restart. Resolve discovered regressions
   before layering recovery on top. This is the first ready task.
2. **Make hardware startup repeatable — T-050.** Shape a bounded macOS-first
   launcher using the native backend and local built UI assets. Default to
   commands disabled and no SITL/UDP input; provide one documented launch
   command, known settings location, useful occupied-port/build/startup errors,
   and clean shutdown that releases serial ownership. A development launcher
   can enable early bench work; clean-machine/offline verification is still
   required before claiming all of T-050. Do not require an installer or
   Windows support for the first local bench experiment.
3. **Make interruptions predictable — T-049.** Implement active-intent retry
   with bounded backoff and cancellation. Re-enumerate identifiable devices;
   require explicit selection for ambiguity or devices without reliable
   identity. Cover unplug/replug, changed device path, startup before attachment,
   brief/long aircraft silence, backend restart, browser reload and deliberate
   release to MissionPlanner. New packets alone refresh instruments; preserve
   context without mixing returning vehicle identities or revalidating old
   mission snapshots. Verify telemetry rate requests resume even when a short
   interruption never causes a vehicle-lost event.
4. **Accept physical USB observation — T-028.** After applicable software
   gates pass, execute the session below and record actual timings, limitations
   and wire/backend/UI evidence. A successful early tilt experiment is useful
   feedback, but does not complete startup/recovery acceptance by itself.
5. **Move to the radio bench — T-031.** Reuse the same UI/profile path with the
   actual ground radio. Record LR900 pair settings and throughput; distinguish
   ground USB loss from aircraft silence with the ground port still open.
   Repeat freshness, recovery and offline checks on powered ground hardware
   before T-032 field observation. Radio configuration, parameter writes,
   mission upload and PID tuning do not block the first USB milestone.

Before claiming T-049/T-050, update their acceptance criteria and commands and
promote them to ready through the normal board workflow. Retain their broader
acceptance obligations; this plan does not mark implementation complete.

## USB session and measurable acceptance

- Start without the controller attached. The normal UI loads and offers
  connection controls with an empty fleet; browser connectivity must not imply
  aircraft connectivity. Repeat once with the controller attached before launch.
- Attach the USB data cable, choose the enumerated controller and actual serial
  settings, and connect. Record device metadata, selected baud, firmware build
  and system/component identity. Confirm firmware independently if it is not
  exposed in the application; do not assume identity/version UI already exists.
- Measure attachment → enumeration → port open → first valid frame → heartbeat
  discovery → fresh attitude separately. Record USB boot time independently of
  application acquisition time. Investigate any failure against the existing
  T-027 identity/recovery bounds; do not silently move the timing origin.
- Hold level, then gently roll left/right and pitch nose up/down, pausing at
  each pose. Compare decoded ATTITUDE with backend samples and displayed
  degrees: sign/axis must agree, numerical differences must be limited to
  display rounding, and the horizon must update without requiring GPS, home,
  a mission, arming or flight-battery power. Proposed UI response target:
  visible change within 500 ms of the corresponding received packet.
- Separate the requested 10 Hz attitude rate from the measured delivered rate.
  Stop ATTITUDE while retaining heartbeat in the harness: the horizon must
  become unavailable according to its actual freshness threshold. Audit the
  T-027 proposal of three message intervals against implementation before
  declaring a pass; resolve any discrepancy explicitly in the contract/tests.
- Unplug/replug, reload the browser, restart the backend, then release/reconnect.
  Target fresh recovery within the existing T-027 10-second bound after link
  availability; explicitly released connections must stay released. Test
  ambiguity with controlled inventory rather than waiting for physical twins.
- Missing GPS/home/battery/peripheral data stays unavailable; reported values
  retain their actual meaning. USB-only power does not prove every peripheral
  is absent or every reported battery value is invalid.
- Capture outbound MAVLink: only the documented observer acquisition traffic
  (heartbeat and addressed interval requests, plus mission reads if explicitly
  exercised) is allowed. No arm/disarm, parameter writes or mission writes.
- Save a short recording and replay it to check that attitude and stale periods
  remain meaningful. Record actual outcomes under T-028, with optional captures
  in `docs/temp/evidence/T-028/<run-id>/`.

## Verification approach

Use existing checks first; add regression tests for faults found, especially
retry cancellation, short-gap rate recovery and stale-state bootstrap. Suggested
baseline commands and the repeatable harness are pinned in T-053. Then use the
target Plane 4.6.3 QuadPlane SITL procedure in
[T-046 validation](../runbooks/validation/t046.md) before physical acceptance.
SITL/PTY evidence cannot establish USB power, driver, cable or controller boot
behavior. No hardware has been opened or exercised while preparing this plan.
