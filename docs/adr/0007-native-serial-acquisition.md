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

### Platform and CGO boundary — 2026-09-16

Develop on macOS and deploy the same application source on Linux. Produce a
separate executable for each supported OS and CPU architecture; a macOS binary
does not run on Linux. Share MAVLink, connection lifecycle and application
logic, isolating OS-specific discovery and serial access behind the transport
and inventory interfaces. Portability is a design requirement, not yet an
executed hardware compatibility claim.

Use the existing `go.bug.st/serial` dependency and gomavlib serial support as
the initial implementation path. This assumes the controller and ground radio
present OS-visible serial ports, which hardware acceptance must confirm.
USB-C describes the physical connection; it does not require raw USB access.
Do not introduce `gousb`/libusb for this serial workflow. Reconsider raw USB
only if observed device requirements demand it.

Permit CGO for the native macOS build: the pinned serial library's detailed
USB enumeration uses Apple's IOKit/CoreFoundation frameworks through CGO.
Core serial reads/writes do not themselves require CGO. Detailed enumeration
supplies available device identity metadata for selection and saved profiles;
missing metadata must remain explicit. See the upstream
[enumerator documentation](https://pkg.go.dev/go.bug.st/serial/enumerator).

Keep the Linux production/container build CGO-free where the selected serial
implementation supports it. The current Dockerfile's `CGO_ENABLED=0` is a
packaging choice, not a project-wide prohibition on native platform bindings.
macOS discovery must not introduce a Linux dependency on Apple frameworks or
libusb. Native macOS builds require a C toolchain and SDK; prefer native
platform builders for releases rather than assuming CGO cross-compilation is
as simple as setting `GOOS`/`GOARCH`. Go race-detector checks use CGO separately
from the production build policy.

T-046 must verify both build boundaries and controlled serial behavior on
macOS and Linux. T-050 owns distribution, prerequisites and the exact supported
OS/architecture matrix. Linux device permissions and container device access
need their own checks; compiling a Linux binary on a Mac does not prove them.

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
