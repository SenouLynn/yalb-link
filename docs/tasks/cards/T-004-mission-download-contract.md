---
id: T-004
title: Complete the read-only mission download contract
status: ready
priority: 0
owner: unassigned
depends_on: none
---

## Motivation and evidence

The flight-HUD/trajectory end-state explicitly distinguishes its prediction
from a commanded route while naming mission UI as a remaining gap. The codec
already decodes `MISSION_COUNT`, `MISSION_ITEM_INT`, and `MISSION_ACK`, and
encodes `MISSION_REQUEST_INT`, but it cannot initiate a download because
`MISSION_REQUEST_LIST` is absent. `proto/gcs/v1/missions.proto` represents
individual protocol payloads but has no shared snapshot result for a backend
service and browser to consume.

## Outcome

The repository has a generated, versioned contract for one read-only mission
snapshot and complete codec support for initiating and continuing a mission
download.

## Scope

- In: `MISSION_REQUEST_LIST` encoding and golden fixture, an ordered mission
  snapshot/result protobuf, generated Go and TypeScript stubs, capability
  matrix coverage, and contract tests.
- Out: transfer orchestration, HTTP routing, UI, mission upload, clear, start,
  set-current, fence, and rally operations.

## Acceptance criteria

- [ ] The codec emits an addressed `MISSION_REQUEST_LIST` for a specified
      vehicle and mission type, covered by a golden MAVLink frame.
- [ ] A shared protobuf represents vehicle identity, mission type, ordered
      mission items, and the observation time of a completed snapshot.
- [ ] The contract represents a completed empty mission without inventing a
      waypoint or treating it as an error.
- [ ] Generated Go and TypeScript code and the codec capability matrix agree
      with the new contract and message family.

## Verification

```sh
make proto
make check-contracts
make check-codec
```

## Open questions

None. This slice defines only a completed snapshot; transfer progress and
failure belong to the coordinator and transport layers.

## Notes

The existing outbound upload encoders do not authorize an upload surface.
Keep the new API read-only and mission-type-specific.
