---
id: T-005
title: Coordinate addressed mission downloads
status: done
priority: 0
owner: unassigned
depends_on: T-004
---

## Motivation and evidence

Mission transaction payloads currently reach the bridge as protocol events and
are logged, but no request registry consumes them. A download must correlate
responses by full vehicle identity and mission type, request each advertised
sequence exactly once, and terminate predictably when the link is incomplete.

## Outcome

A backend service downloads one vehicle's ordinary mission into a validated
snapshot without accepting foreign, mismatched, or incomplete protocol traffic.

## Scope

- In: one in-flight download per vehicle and mission type, addressed writes,
  count/item/ACK correlation, ordered assembly, empty missions, cancellation,
  timeout, and deterministic tests with an injected clock and fake sink.
- Out: HTTP, browser UI, persistence, retries after a terminal failure, uploads,
  partial-result display, fence, and rally downloads.

## Acceptance criteria

- [x] A download sends `MISSION_REQUEST_LIST`, accepts the matching count, and
      requests and assembles exactly the advertised item sequences.
- [x] Zero count returns a successful empty snapshot and sends the protocol
      acknowledgement required to complete the exchange.
- [x] Responses from another vehicle or mission type, responses addressed to a
      different ground station, duplicate items, and out-of-range sequences
      cannot complete or corrupt the transaction.
- [x] Cancellation, timeout, link-write failure, and negative mission ACK each
      produce a distinct terminal error and release the in-flight slot.
- [x] Separate vehicles can download concurrently while duplicate downloads
      for the same vehicle and mission type are rejected deterministically.

## Verification

```sh
go test -race ./internal/mission/... ./internal/bridge/...
make test-go
```

## Open questions

None. Initial downloads are single-attempt transactions; a later card may add a
bounded retry policy using evidence from live links.

## Notes

Do not key correlation by system ID alone. Components and mission types share a
link and must not settle one another's transfers.

T-004 added `target_system`/`target_component` to `MissionCount`, `MissionItem`,
and `MissionAck` so this card can be satisfied at all. Without them a response
this GCS never requested is indistinguishable from one it did: the envelope's
`vehicle_id` says who sent a response, not who it was addressed to, and two
ground stations downloading the same mission type from the same vehicle differ
only in the target. `internal/command/registry.go` already applies exactly this
check to `COMMAND_ACK` and is the pattern to follow.

`contracts/mavlink/mission_count_foreign_v2.bin` is a MISSION_COUNT addressed to
(42, 99) for the rejection test.

Implementation notes:

- `Coordinator.Download(ctx, codec.Target, missionType)` returns a snapshot or a
  terminal error; `Coordinator.Publish` is a `bridge.Sink` and does the
  correlation. Wiring it into `cmd/gcs` is T-006.
- The in-flight key is `(system_id, component_id, mission_type)`. Each of those
  three has a test that fails if it is dropped from the key, and a fourth covers
  responses addressed to another ground station.
- `Timeout` is per response, not per download. A large mission is many round
  trips, and a whole-transfer deadline would fail slow-but-healthy links while
  still tolerating a vehicle that had gone silent.
- The correlation tests were mutation-checked. An earlier version of them passed
  even with the correlation rules removed: a wrongly accepted `MISSION_COUNT`
  still stalls at the first item request and times out, so the timeout alone
  proved nothing. They now assert that no `MISSION_REQUEST_INT` was ever sent,
  which is what distinguishes "ignored" from "believed, then stalled".
- Single attempt by design. A retry cannot tell a dropped request from a busy
  vehicle, and re-requesting a sequence already being answered is how a transfer
  desynchronises. A bounded retry policy remains a later card.
- Verified: `go test -race ./internal/mission/... ./internal/bridge/...`,
  `make test-go`, `go build ./...`, and the full frontend suite. `golangci-lint`
  and `bazel` are not installed in this environment; `internal/mission/BUILD.bazel`
  is hand-written and unbuilt.
