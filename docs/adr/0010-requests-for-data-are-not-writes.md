# 0010 — Requests for Data Are Not Writes

**Status:** Accepted

**Extends:** ADR-0007 §1, whose discovery round trip is unchanged. This names the class that
round trip already belongs to, and schedules the rest of it.

**Evidence:** the 2026-08-19 live SITL run recorded in `docs/changelog/CHANGELOG.md`;
`docs/reference/codec-capability-matrix.md:165,168`;
`docs/adr/0007-firmware-variance-via-capability-negotiation.md` §1 and Consequences.

---

## Context

A backend held six minutes of continuous contact with the pinned `Copter-4.7.0` SITL and
logged one `VEHICLE_DISCOVERED`, zero `VEHICLE_LOST` — so heartbeats never stopped — and
**zero telemetry events**. Nothing had asked the vehicle to send any.

The roadmap schedules the only thing that would ask — `MAV_CMD_SET_MESSAGE_INTERVAL` — at
Tier 8a. Tier 6 exits on "browser shows live TelemetryLog" and Tier 7 on "`ARMING_CHECK`
readable from live ArduCopter". Two gates depend on a capability scheduled two tiers after
them, and neither can pass.

**Two premises held that defect in place, and both were wrong.**

**The first was factual.** Three documents asserted that SITL streams telemetry
unprompted — `tier-1-pure-domain-logic.md:84`, `tier-0-3-adversarial-review.md:366`, and
the comment on `StreamRequestEnable` at `internal/codec/frame.go:135`. The reasoning built
on it was that Tiers 5–6 would look healthy and only first hardware bring-up would be
surprised, which made deferring the fix to Tier 8a survivable. The run falsifies it. SITL
streams nothing either, so the defect arrives at Tier 6 rather than in the field. This also
inverts the objection to setting `SR0_*` rates in the SITL parameter overlay: with neither
SITL nor a field vehicle streaming unprompted, an overlay would *create* the divergence it
was supposed to avoid.

**The second was a category error, and it is the one this ADR exists to fix.** The fix
looked like it crossed Principle 3, "read before write". It does not, because Principle 3
as written — *"read-only proven before any outbound capability"* — does not describe what
this repo does. Three counter-examples, all deliberate and all already decided:

1. **Tier 5 already transmits.** gomavlib emits the GCS `HEARTBEAT` from (255, 190) by
   explicit decision (`order-of-operations.md:44-46`). Outbound, on the first tier with a
   socket.
2. **Tier 7 already transmits.** Its parameter and mission chapters send
   `PARAM_REQUEST_READ`, `PARAM_REQUEST_LIST`, `MISSION_REQUEST_LIST`, `MISSION_REQUEST_INT`
   and `MISSION_ACK` — five of the eleven families in the priority send set. Tier 7 is the
   *read* tier.
3. **An outbound `COMMAND_LONG` is already scheduled at Tier 5.** ADR-0007 §1 gates every
   optional protocol path on `AUTOPILOT_VERSION` (#148), *"requested at discovery"*, and its
   Consequences accept the cost plainly: *"Discovery gains a round trip.
   `MAV_CMD_REQUEST_MESSAGE` for #148 must complete before any capability-gated path is
   decided."* `docs/reference/codec-capability-matrix.md:168` assigns it to **Tier 5**. No
   carve-out was requested and no Principle 3 objection was raised, because none is owed.

The repo has therefore been operating on a boundary it never wrote down: **outbound frames
that ask a vehicle to send data are read-side work.** Tier 7's own overview states it
correctly — *"Nothing in this tier sends a write that changes vehicle configuration"* — and
`port-plan.md:52` enumerates "every write" as arm/disarm, mode change, mission upload and
guided reposition. Principle 3's wording is the outlier.

Meanwhile the cmd-512 round trip that ADR-0007 mandates is **scheduled in no tier file at
all.** It exists in an ADR that is law and in a Reference-class matrix, and nowhere in the
roadmap. That is the real ordering defect, and the stream-request problem is a symptom of
it: both need one thing, an outbound request path at discovery, and nothing owns it.

---

## Decision

### 1. The boundary is vehicle-state mutation, not outbound bytes

A frame is a **write** when it changes what the vehicle is or does after it is processed.
By that test:

| Read-side | Write-side |
|---|---|
| `HEARTBEAT` (GCS liveness) | `PARAM_SET` (#23) |
| `PARAM_REQUEST_READ` (20), `PARAM_REQUEST_LIST` (21) | `MISSION_COUNT` (44) / `MISSION_ITEM_INT` (73) upload |
| `MISSION_REQUEST_LIST` (43), `MISSION_REQUEST_INT` (51) | `MISSION_CLEAR_ALL` (45) |
| `COMMAND_LONG` cmd 512 `MAV_CMD_REQUEST_MESSAGE` | `COMMAND_LONG` cmd 400 `MAV_CMD_COMPONENT_ARM_DISARM` |
| `COMMAND_LONG` cmd 511 `MAV_CMD_SET_MESSAGE_INTERVAL` | `COMMAND_LONG` cmd 176 mode change |
| | `SET_POSITION_TARGET_GLOBAL_INT` (86) |

`MAV_CMD_SET_MESSAGE_INTERVAL` sits on the left for the reason Tier 8 already gave it:
*"Reversible. Affects only telemetry rate. **No vehicle state change.**"* It rides
`COMMAND_LONG`, which is why it was filed under writes — but the envelope is not the
classification. cmd 512 rides the same envelope and is already Tier 5.

**Principle 3 is restated, not weakened.** Read-only proof still precedes every mutation.
The dual-gate safety model (`port-plan.md:52`) is untouched, and it applies to the right
column only, exactly as it did before.

### 2. Tier 5 owns the discovery request round trip

On first `HEARTBEAT` from a `(sysid, compid)` on a link, the GCS sends, addressed to that
link and never broadcast (`order-of-operations.md:65`):

- `COMMAND_LONG` cmd 512 requesting `AUTOPILOT_VERSION` (#148) — ADR-0007 §1, and the
  source of `flight_sw_version` and `uid`/`uid2` that ADR-0009 §2's cache predicate is
  keyed on.
- `COMMAND_LONG` cmd 512 requesting `AVAILABLE_MODES` (#435) — ADR-0007 §2 layer (a). A
  vehicle without it stays on layer (b), the generated dialect enum, which is the designed
  fallback and not an error.
- `COMMAND_LONG` cmd 511 setting an interval for each **periodic** family in the priority
  receive set: `SYS_STATUS` (1), `GPS_RAW_INT` (24), `ATTITUDE` (30),
  `GLOBAL_POSITION_INT` (33), `MISSION_CURRENT` (42), `NAV_CONTROLLER_OUTPUT` (62),
  `VFR_HUD` (74), `BATTERY_STATUS` (147), `EKF_STATUS_REPORT` (193).

**Three of the twelve streaming families are deliberately not requested**, and the reason
is per-message rather than a blanket rule:

- `RADIO_STATUS` (109) is injected by the SiK radio firmware, not produced by the
  autopilot. There is nothing to set an interval on, and it does not appear over a SITL UDP
  link at all — so no gate may require it against SITL.
- `STATUSTEXT` (253) is event-driven. It arrives when the vehicle has something to say; an
  interval is not the right control.
- `HOME_POSITION` (242) is sent on home change and on request. It is requested with cmd 512
  when a consumer needs it, not rate-limited.

Requesting an interval for a family the autopilot does not emit periodically produces a
`COMMAND_ACK` we would have to ignore and no data either way. Naming the three is what
stops the next reader "fixing" the list back to twelve.

**Rates are decided in the tier chapter and verified against SITL, not asserted here.** The
one binding constraint is the uplink arithmetic already on file: the GCS heartbeat
cardinality decision (`order-of-operations.md:45`) protects `RADIO_STATUS.txbuf` as the
back-pressure signal, and stream rates are the other half of that budget.

### 3. No command registry at Tier 5

The request round trip is fire-and-observe. It does **not** wait on `COMMAND_ACK`, does not
enter an in-flight registry and does not write an audit event. The success signal is
telemetry arriving; the failure signal is telemetry not arriving.

This is not a shortcut around Tier 8 Chapter 1 — it is that the registry buys nothing here.
A registry exists to correlate an operator's action with its outcome so a human can be told
what happened. Nobody asked for this; it is bring-up, it is idempotent, and its outcome is
directly observable in the thing it was for.

**Re-send on a timer.** The link is UDP and the request can be lost, in which case a
vehicle in perfect health delivers nothing forever. Re-sending is what makes that
self-correcting. gomavlib's own stream-request module re-sends every 30s
(`node_stream_request.go`, `streamRequestPeriod`); matching that order of magnitude is
sensible and the tier chapter pins the number.

### 4. Tier 8a narrows to the operator-facing capability

Tier 8a keeps `CommandService.SetMessageInterval`, its registry entry, ACK correlation,
role check and audit event — an operator deliberately changing a rate on a live vehicle,
which is an auditable action. It no longer owns first telemetry, because first telemetry is
not an operator action and never was.

### 5. `StreamRequestEnable` stays false

gomavlib's built-in path was the cheap alternative and it is declined on the merits, not on
principle. It sends the deprecated `REQUEST_DATA_STREAM` (#66) for seven coarse stream
groups, which is a different message family — one that would move the
`len(SendFamilies) == 11` pin and put a deprecated message in the send set permanently — and
it gives no per-message control, so the "nine yes, three no" distinction in §2 becomes
unexpressible. Its confirmation event, `EventStreamRequested`, is one the bridge currently
ignores (`internal/bridge/bridge.go:176`), so it would also be the only outbound path in the
system with no observable trace.

The flag's *original* justification is void regardless: the comment at
`internal/codec/frame.go:135` claims SITL streams unprompted, and it does not.

---

## Consequences

**Accepted costs.**

- **`COMMAND_ACK` for 511 and 512 is decoded and dropped.** `internal/codec` decodes it to a
  `ProtocolEvent`, but `vehicle.Event` has no transaction member and the fold discards
  transactions deliberately until Tier 7 owns correlation. So a vehicle that *rejects* a
  request is, at Tier 5, indistinguishable from one that ignored it. Both present as absent
  telemetry, both are corrected by the re-send, and neither is silent — the absence is
  visible in the log. Tier 7 is where this stops being true.
- **A vehicle that never answers #148 stays in the unknown-capability state**, which is
  ADR-0007 §4's designed behaviour, not a regression introduced here.
- **Tier 5 grows a chapter and stops being purely receive-side.** That is the honest shape:
  it was never purely receive-side, because it has emitted a GCS heartbeat since the day it
  opened a socket.
- **Rates are a budget that has to be managed from here on.** Nine families per vehicle at a
  chosen interval is a number that scales with fleet size against a link the roadmap already
  treats as scarce.

**Expected benefits.**

- Tier 6 and Tier 7's exit gates become meetable, and so do ADR-0007's capability path and
  ADR-0009 §2's UID predicate — four defects closed by one chapter, because they were one
  defect.
- The read/write boundary is written down, so the next request-shaped message does not
  reopen it. It was already the operative rule; it just had no text.
- SITL keeps parity with a field vehicle in the property under test. Neither streams until
  asked, and now both are asked the same way by the same code.
- First telemetry stops depending on the command registry, which means it stops depending on
  Tier 8.

**Monitoring.** Two claims to watch. First, that no request-shaped message quietly acquires
a state change — if a cmd 511/512 call ever needs an eligibility gate, this classification
has failed and the message belongs in the right-hand column. Second, that the re-send timer
never becomes the thing that hides a vehicle refusing a request; when Tier 7 lands ACK
correlation, the refusal should surface rather than stay masked by a retry.
