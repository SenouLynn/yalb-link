# 0009 — Parameter Acquisition Paradigms

**Status:** Accepted

**Extends:** ADR-0007 §5, whose mechanism is unchanged. This narrows its vendoring scope
and adds the connection-context split it did not address.

**Evidence:** `docs/research/cockpit-reference-reconciliation.md`;
`docs/adr/0007-firmware-variance-via-capability-negotiation.md` §4–§5.

---

## Context

ADR-0007 §5 settled how parameter *metadata* is versioned and selected, and `306a429`
landed the contract for it. `proto/gcs/v1/parameters.proto` has not changed since. **The
model is stable and has never been reversed.**

What has been reopened at every review is *where the bytes come from*. It moved twice and
was left explicitly open by ADR-0007's 2026-08-18 amendment. Two structural reasons, both
fixable, and neither of them "we keep changing our minds":

**Parameter metadata is the only material in this repo that cannot follow our own
acquisition doctrine.** `scripts/gen_mavlink_fixtures.py` states the rule: *"a fixture
regenerable from a script in this repo is the only kind that satisfies ADR-0001's
hermeticity argument. The predecessor plan sourced these from a sibling checkout that is
not present, not pinned and not reachable by URL or SHA."* Golden MAVLink bytes obey it —
we generate them. Parameter metadata cannot: it is ~2.2 MB per set, authored upstream, and
unproducible by us at any effort. A permanent exception to a rule the repo is otherwise
strict about attracts re-litigation every time someone reads the rule.

**Nothing owned it.** Tier 7 consumes metadata; no tier produces it. A question with a
consumer, no producer and no gate has nothing that can close it, so every review reopens
it by default rather than by disagreement.

Separately, the scope was never bounded. ADR-0007 sized vendoring at ~23 of 105 published
pairs, ~46 MB — a number chosen before the supported vehicle set was decided.

---

## Decision

### 1. "Parameters" is three questions, not one

Conflating them is why the latency argument and the hermeticity argument kept talking past
each other. They have different sources, different change rates and different budgets.

| | Source | Changes when | Latency budget | Lands in |
|---|---|---|---|---|
| **Metadata** — units, range, enum, bitmask | ArduPilot upstream, per firmware version | firmware changes | build time; **never** on a live path | Tier 7 |
| **Values** — what this airframe is set to | the vehicle, over MAVLink | an operator writes one | context-dependent — see §2 | Tier 7 |
| **Writes** — changing one | us → vehicle | operator action | bench only | Tier 8b |

### 2. Two connection contexts, and the discriminator is a predicate — not a mode

**Bench / cold connect.** Configuration, tuning, flashing. Latency is tolerable and
correctness is everything. Run the full `PARAM_REQUEST_LIST` fold with gap detection,
resolve the metadata set, populate the cache. Writes happen here.

**Field reconnect.** A link dropped and recovered, plausibly in the air. Parameters are not
being edited. Serve **cached values keyed on vehicle UID**. Do not re-pull ~1400
parameters. Do not touch the network for anything. No writes by default.

**The discriminator is not "is the vehicle flying."** We cannot know that reliably, and an
operator-set mode is a switch someone forgets. It is a checkable predicate:

> Do we already hold a complete parameter set for this **vehicle UID** at this
> **`flight_sw_version`**?

No → cold path. Yes → reconnect path. Both facts are already in the contract:
`VehicleCapabilities.uid`/`uid2` is hardware identity (ADR-0007 §8 — sysid is
operator-assignable and follows the slot, not the airframe), and `flight_sw_version` is
already the metadata join key. A firmware change invalidates the cache automatically,
which is the correct behaviour and falls out for free.

### 3. Metadata never sits on a live path

Vendored at build time, selected at runtime from data already held. **It is never fetched
at runtime, in either context.** If no set matches the vehicle's firmware, the UI degrades
visibly — nearest vendored set with `is_exact_match = false`, or no annotation at all —
and never blocks, never fetches, never guesses.

This is what makes the source question genuinely low-stakes: a build-time input cannot
affect field behaviour, so choosing it wrong costs a rebuild, not an incident.

### 4. Vendoring scope follows the supported vehicle set

We support **Copter and Plane**. Therefore we vendor **two** metadata sets, matching the
pinned SITL firmware exactly:

| Vehicle | SITL pin | Metadata set |
|---|---|---|
| Copter | `Copter-4.7.0` | `Copter-4.7` |
| Plane | `Plane-4.6.3` | `Plane-4.6` |

Roughly 4 MB, not ~46 MB. ADR-0007 §5's mechanism — per minor line, runtime-selected,
mismatch surfaced — is unchanged; only its sizing is narrowed, because that sizing was
chosen before the vehicle set was.

**Deliberately two different minor lines.** It is not tidiness lost, it is coverage gained:
Copter 4.7 exercises ADR-0007's mode layer (a), `AVAILABLE_MODES` (#435), which ArduPilot
implements from 4.7.0; Plane 4.6 exercises layer (b), the generated dialect enum fallback.
Pinning both to one line would leave one layer permanently unexercised in SITL. It also
makes version selection non-trivial, which is the only way `is_exact_match` gets tested
against something other than a perfect hit.

**Adding a vehicle type is one list, stated once:** a metadata set, a SITL service, and a
`waf` target. See `order-of-operations.md`, supported vehicle set.

### 5. The reconnect fast path is already in the contract, and stays unbuilt

`MAV_PROTOCOL_CAPABILITY_FTP` (32) gates the `@PARAM/param.pck` MAVFTP path — a complete
parameter set as one file transfer instead of ~1400 `PARAM_VALUE` messages. Where the bit
is declared, that is the reconnect refresh; where it is not, `PARAM_REQUEST_LIST` with
Tier 7's gap-detection fold. A capability check, not a firmware branch (ADR-0007 §1).

**Not implemented here, and that is the decision.** It has no caller. Recorded so the
reconnect paradigm has a known answer rather than an open question.

### 6. Ownership, trigger, and what "closed" means

The converter lands in **Tier 7**, whose `ParametersPanel` is its first consumer — per
ADR-0007 §8's rule that a port lands in the tier that first has a caller, and that *"an
interface with no implementations **and** no callers is speculative API design."*

**Until Tier 7 begins, the source question is closed, not open.** The remaining choice —
`ArduPilot/ParameterRepository` pinned by commit versus `autotest.ardupilot.org` per-tag
URLs — is made by whoever writes the converter, against a real Bazel target, with the
trade-off already tabled in ADR-0007's amendment. Reopening it before then requires new
evidence that changes the decision, not a new preference.

That sentence is the operative one. The churn was never disagreement about the answer; it
was an unowned question with no gate, and those get reopened by default.

---

## Consequences

**Accepted costs.**

- Two vendored sets means a vehicle type we have not scoped has no metadata at all. That
  is correct and visible — `is_exact_match` and an absent set both degrade loudly — but it
  is a real limitation, not a temporary one.
- The reconnect path serves values that may be stale if someone changed a parameter via
  another GCS while we were disconnected. Accepted: the alternative is a multi-minute pull
  on every link recovery, which is worse in the case that matters. A firmware-version
  change still invalidates automatically.
- MAVFTP stays unbuilt, so first-connect on a slow link remains slow until Tier 7 or later.

**Expected benefits.**

- The scope question and the source question stop being coupled. Narrowing to two vehicles
  cut the fetch tenfold and made the source choice cheap enough to defer honestly.
- The bench/field split is a predicate over data we already carry, so no new contract, no
  new mode, and no operator switch to forget.
- The question has an owner and a trigger, which is what stops it being reopened.

**Monitoring.** The claim to watch is that no runtime path ever fetches metadata over the
network. If one appears, §3 has failed and the contract is carrying the wrong thing.
