# Reference

Normative external format specifications that our contracts must interoperate with.

Distinct from `docs/research/` — research captures what we learned about other projects
and may lead nowhere; a reference document defines something an exit gate tests against.
Every claim here carries its provenance, and unverified claims are marked as such. See
ADR-0008 §1 for the lifecycle of this class, and §3 for why documents here outrank prose
elsewhere when the two disagree about what the system does.

| Document | Purpose | Enforced by |
|---|---|---|
| [codec-capability-matrix.md](codec-capability-matrix.md) | Decode/encode coverage for `internal/codec` — every row names an evidence artifact | `scripts/check-matrix.sh` and `TestMatrixCoverage`, both in `make gate-tier-2` |
| [mission-interchange-formats.md](mission-interchange-formats.md) | QGC `.plan` and ArduPilot `.waypoints` mapped field-by-field onto `MissionItem` | Tier 7 exit gate (not yet mechanized) |
