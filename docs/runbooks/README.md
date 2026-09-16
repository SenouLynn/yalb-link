# Runbooks

Runbooks contain repeatable operational procedures, not architecture plans.

- [dev-setup.md](dev-setup.md) — prerequisites, tests, generation, and local SITL

Recorded validation procedures and results (raw captures are optional):

- [T015](validation/t015.md) — T-015 evidence — decoded flight state the display no longer discards
- [T017](validation/t017.md) — T-017 workspace acceptance — 2026-09-09
- [T019](validation/t019.md) — Copter motion baseline, independent measurements and fault/replay procedure
- [T021](validation/t021.md) — T-021 Compose recording persistence
- [T022](validation/t022.md) — T-022 fleet overview acceptance — 2026-09-09/10
- [T030](validation/t030.md) — T-030 observer startup without internet — 2026-09-14
- [T052](validation/t052.md) — T-052 backward track/Flown-distance repeat — 2026-09-15

See the [temporary artifact workflow](../temp/README.md) for capture and cleanup.
Reusable mission validation helpers live in `scripts/validation/`.
- [aeronautics.md](aeronautics.md) — standalone calculator checks in `aeronautics/`
