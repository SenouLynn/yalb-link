---
id: T-027
title: Capture the Plane 4.6.x QuadPlane acceptance profile
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

Record exact firmware/build, board/frame, host and link configuration so compatibility claims have a target. See the [operator usage plan](../operator-usage-plan.md) for current evidence and scenario definitions.

## Outcome

Capture the Plane 4.6.x QuadPlane acceptance profile with explicit evidence and remaining limits.

## Scope

Capture controller identity and baseline parameters using the available established tool; record unknowns explicitly. Choose a repeatable target SITL configuration or document any simulation mismatch. Define E2E-01 through E2E-04 tolerances before execution.

## Acceptance criteria

- [x] Profile distinguishes observed hardware facts from assumptions and names the gaps only hardware can close.
- [x] Relevant Plane/QuadPlane SITL checks and a reproducible setup are specified; Copter 4.7 is not treated as equivalent.

## Verification

Manually inspect controller identification and exported baseline; review version and frame evidence against the proposed SITL setup. No application writes or flights.

```sh
docker compose --profile target-profile up -d --build --wait \
  gcs-backend ardupilot-sitl-quadplane-tri-4
curl -s -N --max-time 5 http://localhost:8080/api/events   # observe systemId=4
```

Full evidence and the E2E-01..04 tolerance definitions are in the
[T-027 runbook](../../runbooks/validation/t027.md).

## Open questions

Exact firmware patch/build, host OS, radio settings and USB power availability require operator/device evidence.

## Notes

Planned, not executed against the real aircraft. Dependencies sequence
outcomes, not proof of hardware compatibility.

Completed 2026-09-16 as a SITL-verified profile, not hardware acceptance.
ArduPilot's own tooling (`vehicleinfo.py`) defines `quadplane-tri` as two
layered default-param files (`quadplane.parm` then `quadplane-tri.parm`);
`docker/sitl/Dockerfile` only supported one, so it and `entrypoint.sh` were
changed to accept a space-separated `DEFAULT_PARAMS_PATH` list, copied at
build time with an ordered manifest that `--defaults` (which ArduPilot's
`AP_Param` natively accepts as comma-separated) consumes directly.
`docker-compose.yml` gained `ardupilot-sitl-quadplane-tri-4` under a new
`target-profile` profile (`Plane-4.6.3`, sysid 4).

Verified via pymavlink and the real backend/UI event stream (no application
code changed): `AUTOPILOT_VERSION` reports 4.6.3 exactly, `Q_ENABLE=1` and
`Q_FRAME_CLASS=7` (TRI) confirm the tricopter QuadPlane frame class, and
`GET /api/events` shows the vehicle discovered and streaming telemetry
through the existing fleet pipeline with no special-casing needed. A benign
`Warning model expected 4 motors and got 3` startup message is explained in
the runbook and does not indicate a broken configuration.

This verification run recreated the already-running `yalb-backend` container
from unrelated prior work (up ~19-23h), briefly dropping any live browser
connection; recordings were unaffected (separate volume). The added SITL
container was removed after verification.

Hardware gaps this card cannot close (exact 4.6.x patch/build, board
`AUTOPILOT_VERSION` identity, host OS, LR900 serial settings/throughput, USB
power peripheral availability) remain explicit open questions requiring the
physical controller — T-028/T-029/T-030/T-031/T-032 depend on that evidence,
not on this profile substituting for it.
