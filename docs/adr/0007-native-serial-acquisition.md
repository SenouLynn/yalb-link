# ADR 0007: Native in-process serial acquisition

Status: Proposed — direction settled; [T-046](../tasks/cards/T-046-serial-acquisition.md)
is the accompanying implementing experiment and has not run yet.

## Context

[ADR 0006](0006-operator-connection-readiness.md) makes install, select, save,
connect and recover from a USB controller or ground radio the next operator
milestone, and names "native serial ingress versus a managed adapter" as an
open implementation choice. The executable currently opens one UDP socket
(`internal/codec/frame.go`, `codec.NewNode`); no serial device is reachable
from the application. ADR 0006 already rejects one candidate answer: "an
external serial-to-UDP adapter may be useful experimentally, but a manually
configured bridge does not by itself satisfy the accepted installation and
connection journey."

[T-045](../tasks/connection-contract.md) needs this settled before its
`connection.Manager` and `Device`/`Inventory` interfaces can commit to a
shape, because the two directions imply different backend responsibilities.

## Decision

The backend acquires serial MAVLink natively in-process — enumerating,
opening, and reading OS-visible serial devices itself — rather than requiring
the operator to run a separate serial-to-UDP adapter process as the supported
path. An external adapter remains usable experimentally, exactly as ADR 0006
already allows, but the application must never assume one exists and no
supported journey depends on the operator configuring one.

## Consequences

- Cross-platform device enumeration, port permissions, busy/ownership
  detection, and hot-plug/removal detection become backend responsibilities,
  not something an external tool absorbs on the application's behalf.
- Packaging ([T-050](../tasks/cards/T-050-local-observer-launch.md)) must ship
  a build capable of serial access on each supported host, rather than only
  a UDP-listening binary plus a documented external adapter.
- No new external process or service dependency is required for baseline
  bench or field hardware use.
- The connection contract's `Device`/`Inventory`/`Manager` shapes
  ([T-045](../tasks/connection-contract.md) §5) are written against this
  direction; reversing it would change that interface boundary, not just an
  implementation behind it.
- `internal/codec.FrameSource` gains at least one non-UDP implementation.
  Whether it multiplexes through the existing `codec.Node`/`gomavlib.Node` or
  runs alongside it is left to [T-046](../tasks/cards/T-046-serial-acquisition.md),
  per the open engineering question in [T-045](../tasks/connection-contract.md) §5.

## Verification

[T-046](../tasks/cards/T-046-serial-acquisition.md) implements this against a
controlled pseudo-terminal or platform-equivalent fake serial source, proving
framing, port closure, error handling, and multiple vehicle identities without
any external adapter process. Actual USB/radio hardware acceptance remains
[T-028](../tasks/cards/T-028-usb-observer.md) and
[T-031](../tasks/cards/T-031-radio-ground-acceptance.md); this decision is
adopted before either has run.
