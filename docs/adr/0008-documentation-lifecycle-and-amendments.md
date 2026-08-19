# 0008 — Documentation Lifecycle and Amendments

**Status:** Accepted

**Context for:** every document under `docs/`. This ADR governs how ADRs themselves
change, which is why it is an ADR rather than a README convention — a rule binding law
cannot sit below it.

---

## Context

This repository is documentation-heavy by design: contracts, tier plans and ADRs are the
portable unit, and generated code is an adapter. That makes document decay a first-class
failure mode rather than a tidiness concern. Two distinct decays have already been
observed, and they fail differently.

**Unbounded growth.** ADR-0007 is 311 lines and gained a 60-line amendment two days after
being accepted. `CHANGELOG.md` is past 450 lines with five sections under `[Unreleased]`.
Neither is wrong; both are on a trajectory where the next reader skims instead of reads.

**Silent divergence, which is worse.** `order-of-operations.md:51` lists HEARTBEAT among
"13 streaming families → `TelemetryEvent`". `telemetry.proto` has no heartbeat variant,
`docs/reference/codec-capability-matrix.md` says so explicitly, and the codec's dispatch entry is
nil. The implementation discovered the truth and the canonical document was never
corrected. A reader who consults the authoritative file and then the matrix finds a
contradiction with no stated reason, and concludes the decision is still moving. **It is
not — it was settled once and written down in only one of the two places.** This is the
mechanism by which settled decisions come to feel unsettled, and it is the specific thing
this ADR exists to stop.

There was also no convention for amending an Accepted ADR. One was invented ad hoc on
2026-08-18 for ADR-0007. This ratifies a convention rather than leaving the next one to
improvisation.

---

## Decision

### 1. Seven document classes, each with one lifecycle

| Class | Path | Mutability | Where its history lives |
|---|---|---|---|
| **ADR** | `docs/adr/` | Immutable once Accepted, except a bounded Amendments section (§2) | The supersession chain |
| **Roadmap / tier** | `docs/roadmap/` | Living — edited in place, no amendment log | `CHANGELOG.md` |
| **Reference** | `docs/reference/` | Living, but every claim carries provenance and a verification date | The provenance lines themselves |
| **Research** | `docs/research/` | **Frozen at write time.** Never retro-edited | A reconciliation document |
| **Runbook** | `docs/runbooks/` | Living | — |
| **Changelog** | `docs/changelog/` | Append-only, newest first | Itself |
| **WIP** | `docs/wip/` | Temporary. Must graduate to another class or be deleted | — |

The split that matters is **frozen versus living**. A research document records what was
believed on a date; editing it destroys the ability to ask "what did we know when we
decided this?" A tier file records what we intend to build next; an amendment log on it
would be noise. Applying one policy to both is what makes documentation feel either stale
or churning, depending on which way the compromise fell.

`docs/wip/` is deliberately the only class with an expiry. A document with no owner and no
lifecycle is where divergence breeds.

### 2. ADR amendments

1. **An Accepted ADR's Context and Decision sections are immutable.** Do not edit them to
   reflect facts learned later. The record of what was decided, on what evidence, is the
   artifact.
2. **New facts that do not change the decision** go in a dated block appended at the end:

   ```
   ## Amendment (YYYY-MM-DD) — <scope>
   ```

   Every amendment states three things, in this order: **what was wrong or new**, **what
   did not change**, and **the consequence**. The middle one is not optional — an
   amendment that does not say what still stands reads as a reversal, which is exactly the
   confusion this ADR is about.
3. **If the decision itself changes, do not amend — supersede.** Write a new ADR carrying
   `Supersedes: NNNN`. The old one gets `Status: Superseded by NNNN` plus a single pointer
   line, and its body is never touched.
4. **Amendment budget: three.** A fourth is the signal that the ADR has drifted from its
   own text and should be superseded by a rewrite that folds the amendments in. This is
   the bound that keeps ADRs readable; without a number it is a matter of taste and the
   answer is always "one more".
5. **The Status line names the amendments**, so a reader knows the body is qualified
   before reading it.

### 3. Executable artifacts outrank prose

The authority order — ADRs → `order-of-operations.md` → tier files → `port-plan.md` —
governs **intent**: what we mean to do, and who wins when two plans conflict.

It does not govern **fact**. Where a document and a test-enforced artifact disagree about
what the system does, **the executable artifact is authoritative and the document is a
defect**. The executable artifacts are the capability matrix (enforced by
`TestMatrixCoverage`), `scripts/check-*.sh`, the golden fixtures in `contracts/mavlink/`,
and the `.proto` files themselves.

This is not a demotion of the roadmap. It is the observation that a claim nothing executes
has no mechanism for staying true, which is the same argument this repo already accepted
for generated constants over hand-typed tables (ADR-0007 §3).

### 4. Divergence from ported material carries its reason inline

`order-of-operations.md` was derived by reading the predecessor, `flight-path-hud`, for
what to port. That predecessor began as **read-only MAVLink**, so parts of it encode
assumptions this repo does not share. The capability matrix, by contrast, was written to
drive *this* repo and is test-enforced.

Divergence from ported material is therefore expected and healthy — but **an unexplained
divergence is indistinguishable from a mistake.** Where this repo deliberately departs
from the predecessor's shape, the reason is written at the point of divergence, not left
to be inferred. Absent a stated reason, a divergence is a defect and gets fixed toward the
executable artifact.

Documents that inherit assumptions from elsewhere say so near the top, so a reader knows
which claims are load-bearing decisions and which are inherited defaults.

### 5. Changelog

Newest first under `[Unreleased]`; on release, sections collapse under a version heading.
Entries are **append-only**: a shipped entry is never rewritten, and a correction is a new
entry that names the old one. The changelog is the history mechanism for every living
class, which is what lets tier files be edited in place without losing the record.

---

## Consequences

**Accepted costs.**

- A fourth amendment forces a rewrite, which is real work at an inconvenient moment. That
  is the point: the alternative is an ADR whose body and tail disagree.
- Provenance lines and verification dates in `docs/reference/` are friction on every edit.
  They are what makes the difference between a reference and a recollection.
- Freezing `docs/research/` means known-wrong statements stay in the tree. They are marked
  by the reconciliation document that supersedes them, not deleted — the wrong belief is
  part of the record of why a decision looked reasonable.

**Expected benefits.**

- A reader who finds a document disagreeing with the code now has a rule that resolves it,
  instead of concluding the decision is in flux.
- ADRs stay bounded, so "read the ADRs" remains realistic advice for a new contributor or
  agent.
- Deliberate divergences stop looking like drift, which is the specific complaint that
  prompted this ADR.

**Monitoring.** The claim to watch is that no document accumulates a fourth amendment
without being superseded, and that no executable artifact disagrees with a document for
longer than it takes to notice. The HEARTBEAT divergence in `order-of-operations.md` is
corrected in the same change that adds this ADR; it is the worked example, not a
hypothetical.
