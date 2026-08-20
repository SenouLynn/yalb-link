# Reference

Normative external format specifications that our contracts must interoperate with.

Distinct from `docs/research/` — research captures what we learned about other projects
and may lead nowhere; a reference document defines something a gate tests against. Every
claim here carries its provenance, and unverified claims are marked as such. Where a
document here and an executable artifact disagree about what the system does, the
artifact wins.

| Document | Purpose | Enforced by |
|---|---|---|
| [decisions.md](decisions.md) | Closed decisions — the decision plus the fact that makes it checkable | Reopening one takes an ADR |
| [codec-capability-matrix.md](codec-capability-matrix.md) | Decode/encode coverage for `internal/codec` — every row names an evidence artifact | `scripts/check-matrix.sh` and `TestMatrixCoverage`, both in `make gate-tier-2` |
| [mission-interchange-formats.md](mission-interchange-formats.md) | QGC `.plan` and ArduPilot `.waypoints` mapped field-by-field onto `MissionItem` | Not yet mechanized |
