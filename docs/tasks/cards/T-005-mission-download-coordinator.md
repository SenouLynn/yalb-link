---
id: T-005
title: Coordinate addressed mission downloads
status: backlog
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

- [ ] A download sends `MISSION_REQUEST_LIST`, accepts the matching count, and
      requests and assembles exactly the advertised item sequences.
- [ ] Zero count returns a successful empty snapshot and sends the protocol
      acknowledgement required to complete the exchange.
- [ ] Responses from another vehicle or mission type, duplicate items, and
      out-of-range sequences cannot complete or corrupt the transaction.
- [ ] Cancellation, timeout, link-write failure, and negative mission ACK each
      produce a distinct terminal error and release the in-flight slot.
- [ ] Separate vehicles can download concurrently while duplicate downloads
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
