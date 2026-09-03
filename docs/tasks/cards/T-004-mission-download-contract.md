---
id: T-004
title: Complete the read-only mission download contract
status: done
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

- [x] The codec emits an addressed `MISSION_REQUEST_LIST` for a specified
      vehicle and mission type, covered by a golden MAVLink frame.
- [x] A shared protobuf represents vehicle identity, mission type, ordered
      mission items, and the observation time of a completed snapshot.
- [x] The contract represents a completed empty mission without inventing a
      waypoint or treating it as an error.
- [x] Generated Go and TypeScript code and the codec capability matrix agree
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

Implementation notes:

- `mission_type` is a MAVLink 2 extension field. At `MAV_MISSION_TYPE_MISSION`
  it is trimmed off the wire entirely, so `mission_request_list_out.bin` has a
  two-byte payload and the receiver's default supplies the zero. A v1 sender and
  an ordinary-mission v2 sender are therefore indistinguishable on this message.
- `MissionSnapshot` describes only completed downloads. Progress and failure are
  deliberately absent so `items` can be trusted as a whole mission; T-005 and
  T-006 carry terminal errors out of band rather than in the snapshot.
- A completed empty mission is `items` absent with `observed_at` present. The
  Go and TypeScript tests assert on the protobuf-JSON wire form, since that is
  what T-006 serves and T-007 reads, and an unfilled message encodes to `{}`.
- `internal/mission` is contract tests only for now; T-005 adds the library.
- Amended after review, before T-005 started. Two defects in the pre-existing
  mission response contract were in scope here because T-005 depends on it:
  - `MissionCount`, `MissionItem`, and `MissionAck` discarded
    `target_system`/`target_component`, which would have made T-005's promise to
    reject foreign traffic untestable. Added to the contract and populated by
    the decoders, matching `CommandAck`.
  - `MISSION_ITEM_INT.x/y` were decoded with the global degE7 scale regardless of
    frame. Local and body frames are metres x 1e4, so local items were reported
    1000x too small. The decoder now selects the scale from `frame` and passes
    non-positional frames through unscaled.
- Local gates run: `make proto`, `make check-contracts`, `make check-codec`,
  `make check-matrix`, `go build ./...`, `go test -race ./...`, and the frontend
  typecheck, lint, and vitest suites. `golangci-lint` and `bazel` are not
  installed in this environment, so `make check-codegen` was verified only by
  its non-Bazel steps.
