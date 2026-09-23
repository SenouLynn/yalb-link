# Local connection service contract

Defined by [T-045](cards/T-045-connection-contract.md), required by
[ADR 0006](../adr/0006-operator-connection-readiness.md). This specifies interfaces for [T-046](cards/T-046-serial-acquisition.md)
through [T-050](cards/T-050-local-observer-launch.md) to implement
independently against. T-046 implements inventory, acquisition, HTTP actions and
SSE bootstrap; its [runbook](../runbooks/validation/t046.md) records executable
evidence. T-047 implements profile storage, identity matching and startup
resolution; [ADR 0010](../adr/0010-connection-profile-persistence.md) records
its costly-to-reverse choices. Connection controls and retry policy remain
later work ([T-048](cards/T-048-connection-controls.md)/[T-049](cards/T-049-connection-recovery.md)).

Read [ADR 0006](../adr/0006-operator-connection-readiness.md) first for the
operator journeys this contract must satisfy. This document does not restate
them; it maps to them in [§10](#10-journey-mapping).

## 1. Scope and non-goals

In scope: device inventory shape, connection lifecycle and status vocabulary,
the HTTP/SSE surface, profile persistence's interface boundary, startup and
retry policy, and how a connection relates to the vehicles it carries.

Out of scope: the serial transport implementation ([T-046](cards/T-046-serial-acquisition.md)),
profile storage schema ([T-047](cards/T-047-saved-connections.md)), UI
composition ([T-048](cards/T-048-connection-controls.md)), the exact recovery
timing bounds ([T-049](cards/T-049-connection-recovery.md), which measures
them), packaging and host OS commitment ([T-050](cards/T-050-local-observer-launch.md)),
offline imagery ([T-030](cards/T-030-offline-observer.md)), and parameter
inspection ([T-029](cards/T-029-parameter-inspection.md)).

## 2. Host scope and acquisition direction

**Assumption, not confirmed target evidence:** initial development and
packaging target macOS and Linux, matching this repository's current
development host and the cross-platform reach of Go serial libraries and
gomavlib. Windows is deferred, not rejected. [T-027](cards/T-027-target-profile.md)
must confirm the actual operator host before [T-050](cards/T-050-local-observer-launch.md)
commits to a platform list.

The [ADR 0007 platform boundary](../adr/0007-native-serial-acquisition.md)
clarifies the implementation policy: develop on macOS and deploy shared source
as separate OS/architecture executables. Permit CGO for macOS detailed device
discovery; preserve a CGO-free Linux production build. T-046 verifies both
platforms and T-050 defines the tested distribution matrix. This policy does
not itself establish hardware support.

**Decision, settled here:** the backend acquires serial MAVLink natively
in-process, not by requiring an operator-run serial-to-UDP adapter. This is
costly to reverse (it shapes packaging, permissions, and the API below), so it
is recorded as [ADR 0007](../adr/0007-native-serial-acquisition.md) rather than
only here.

## 3. Vocabulary

- **Device** — one OS-enumerated candidate (a serial port) or the fixed UDP
  development endpoint. Not yet connected.
- **Settings** — caller-chosen parameters for opening a device (baud rate,
  for serial).
- **Connection** — a configured acquisition path: a device plus settings, with
  a lifecycle state. A connection is not a vehicle identity: [ADR 0006](../adr/0006-operator-connection-readiness.md)
  is explicit that one connection can carry multiple reporting vehicles, and a
  vehicle is addressed by MAVLink system/component ID, not by which connection
  last carried it.
- **Profile** — a saved, named connection target (for example "Bench
  controller", "Field radio") that survives restarts. Profiles select an
  acquisition path; they never grant command capabilities
  ([ADR 0006](../adr/0006-operator-connection-readiness.md)).

These terms are deliberately distinct from the frontend's existing
`StreamEvent` `'connection'` kind (`frontend/src/stream/events.ts`), which
means only "the browser currently holds the backend SSE stream." That is
backend/browser reachability — layer 1 below. A device connection is layers
2 through 4. Conflating them is exactly what [ADR 0006](../adr/0006-operator-connection-readiness.md)
forbids ("browser connectivity must not imply aircraft connectivity"), so the
new SSE event introduced in [§6](#6-sse-event) uses a different name
(`acquisition`, not `connection`) to keep the collision from being possible in
code, not just in prose.

## 4. The five evidence layers

[ADR 0006](../adr/0006-operator-connection-readiness.md) requires distinguishing
backend availability, device presence, port access, valid MAVLink, vehicle
liveness, and reading freshness. Three of these already exist; three are new:

| Layer | Question | Status |
|---|---|---|
| 1. Backend/browser reachability | Does the browser hold the SSE stream? | Exists — `StreamEvent{kind:'connection'}`, `stream.Hub` |
| 2. Device presence | Does the OS currently see this device? | **New** — `Inventory`, [§5](#5-backend-interfaces) |
| 3. Port access | Did opening it succeed? | **New** — `Status.State`, [§5](#5-backend-interfaces) |
| 4. Valid MAVLink | Are bytes parsing as MAVLink frames? | **New** — `Status.State` |
| 5. Vehicle liveness | Is a HEARTBEAT within `codec.HeartbeatTTL`? | Exists — `vehicle.Fold`, `FleetEventType` |
| 6. Reading freshness | Is a given family within its freshness window? | Exists — `LastMsgSeenMs`, `isFamilyFresh` |

A connection's `Status.State` ([§5](#5-backend-interfaces)) covers layers 2
through 4. It must never be inferred from layers 5 or 6: a device that is
present, open, and parsing valid MAVLink but has no vehicle within
`HeartbeatTTL` is `REPORTING` at the connection layer with zero live vehicles,
not `INTERRUPTED`. Interruption means valid frame traffic itself stopped for
60 seconds, independently of vehicle heartbeats. This resolves the original
§4/§8 conflict: valid frames without HEARTBEAT prove connection reporting, not
vehicle liveness.

## 5. Backend interfaces

Proposed Go shapes for [T-046](cards/T-046-serial-acquisition.md) to implement.
Illustrative, not binding on naming; binding on the distinctions they encode.

```go
package connection // proposed package; does not exist yet

// Kind distinguishes the fixed UDP development endpoint from an
// OS-enumerated serial device.
type Kind int

const (
    KindUnspecified Kind = iota
    KindUDP
    KindSerial
)

// Device is one candidate the operator can select.
type Device struct {
    // ID is stable within one Inventory snapshot. It is not guaranteed
    // stable across OS re-enumeration; identity resolution across restarts
    // is a Profile concern (§9), not a Device concern.
    ID           string
    Kind         Kind
    Path         string // e.g. /dev/tty.usbserial-XXXX, COM3
    Description  string // OS-reported product string, when available
    SerialNumber string // when the adapter exposes one; never fabricated
}

// Settings are the caller-chosen parameters for opening a device.
type Settings struct {
    BaudRate int // serial only; ignored for KindUDP
}

// Inventory enumerates currently visible devices. Implementations must not
// claim a generic serial adapter is a specific radio model — Description and
// SerialNumber carry only what the OS actually reports.
type Inventory interface {
    List(ctx context.Context) ([]Device, error)
}

// State is a connection's layer 2-4 status. See §8 for the full machine.
type State int

// Status is one connection's current evidence.
type Status struct {
    ID       string
    Device   Device
    Settings Settings
    State    State

    // DetailedError reports observed access/transport failures and inventory
    // removal evidence (permission denied, device busy, missing). It never
    // guesses an untested cause: not baud mismatch, not "aircraft is off".
    DetailedError string

    OpenedAtMs int64

    // VehicleKeys are the routes.Key values currently attributed to this
    // connection's link. A connection exposes which vehicles it carries
    // without itself being a vehicle identity (§3).
    VehicleKeys []routes.Key
}

// Manager owns every configured connection: the fixed UDP development path
// (when enabled) plus zero or more serial connections the operator has
// selected. Manager is the seam T-046 implements and T-048 drives.
type Manager interface {
    // Connections reports every configured connection and its current status.
    Connections() []Status

    // Connect opens a device under settings and returns its connection id.
    // Connect is idempotent for an already-open identical target: it returns
    // the existing id rather than opening a duplicate device twice.
    Connect(ctx context.Context, device Device, settings Settings) (id string, err error)

    // Disconnect explicitly releases a connection. Explicit disconnect
    // cancels pending retries (§9); the connection does not reopen until
    // Connect is called again. Disconnecting an id already gone is a no-op
    // success, matching codec.Node.Close's idempotence.
    Disconnect(ctx context.Context, id string) error

    // Subscribe follows status changes, bootstrapped like stream.Hub.Subscribe:
    // registration and the initial snapshot happen under one lock, so no
    // transition between them is ever missed by a new subscriber.
    Subscribe() (<-chan Status, func())
}
```

**Resolved by T-046:** `connection.Manager` multiplexes one codec node per
opened transport into a single bridge, route table and fleet fold. Serial
ports open synchronously, then use a one-shot gomavlib custom client; this
avoids gomavlib serial auto-retry taking ownership of T-049 policy. Serial
channel labels include an open generation, so old addressed routes cannot
become writable after reopen. Disconnect cancels that source, releases the
port and forwards channel retirement without closing the shared event stream.

**Original engineering question:** how a serial
`Device`'s frames reach the existing `bridge.Bridge`. `bridge.Config.Source`
is one `codec.FrameSource` today, and `codec.NewNode` takes a fixed
`[]gomavlib.EndpointConf` at `Initialize`, which does not obviously support
adding or removing endpoints on a running node. Two directions are open: (a)
extend `codec.Node` to accept endpoint changes after start, keeping one shared
`routes.Table` and one `bridge.Bridge`; or (b) run one `codec.FrameSource` /
`bridge.Bridge` pair per connection, sharing the same `routes.Table` and
`vehicle` fold keyed by `(SysID, CompID)` through a common `Sink`. Either must
preserve the existing guarantee that `routes.Table.ForgetLink` drops exactly
the routes a closed connection owned, and neither may change `codec.FrameSource`'s
existing UDP behavior.

## 6. SSE event

Add one event name to the vocabulary in `internal/stream/hub.go`:

```go
// EventAcquisition carries a Status transition for one connection. Named
// distinctly from the frontend's transport-only 'connection' kind (§3): this
// event is about device/MAVLink evidence, never about whether the browser
// holds the SSE stream.
EventAcquisition = "acquisition"
```

`Hub` retains the latest `Status` per connection id and bootstraps a new
subscriber with it, exactly like `FleetEvent` and `TelemetryEvent` today — a
browser that connects late must see every currently configured connection's
state, not just changes from that point on.

Frontend consumption (`frontend/src/stream/events.ts`) adds a fourth
`StreamEvent` variant:

```ts
| { kind: 'acquisition'; event: ConnectionStatus; receivedAtMs: number }
```

kept syntactically distinct from the existing:

```ts
| { kind: 'connection'; connected: boolean; receivedAtMs: number }
```

## 7. HTTP API

Following the action-verb convention `internal/command` and
`internal/recording` already use (`POST /api/commands/arm`,
`POST /api/recordings/start`), not the resource-ID convention
`internal/mission` uses — connect/disconnect are stateful actions with
side effects, closer in kind to arm/disarm than to a static download.

```
GET    /api/connections                 configured connections + Status (§5), including
                                         the UDP development entry when enabled and one
                                         entry per saved profile, keyed by profile id
GET    /api/connections/devices         current OS device inventory (Inventory.List)
POST   /api/connections/connect         body {device_id|profile_id, device_id?, settings?} -> opens;
                                         200 + Status; 404 if device_id/profile_id no longer
                                         resolves; 409 if busy/owned elsewhere or AMBIGUOUS
                                         (§8) with no explicit device_id disambiguating it;
                                         profile_id alone auto-resolves by identity (§9);
                                         profile_id + device_id is the explicit-selection path
                                         for AMBIGUOUS/DEVICE_MISSING; omitted settings default
                                         to the profile's saved settings
POST   /api/connections/disconnect      body {id} -> explicit release (Manager.Disconnect);
                                         for a profile-backed id this persists RELEASED intent
GET    /api/connections/profiles        saved profiles (T-047, ADR 0010)
POST   /api/connections/profiles        body {id?, name, device_id, settings} -> save or
                                         update; device identity is always captured fresh
                                         from current inventory, never from the caller
DELETE /api/connections/profiles/{id}   remove a profile
```

Bodies follow `internal/command/http.go`'s conventions: `application/json`
required, `http.MaxBytesReader`-bounded, same-origin checked
(`guard`/`decodeBody`). Operator commands stay gated by
`command.ResolveEnabled`; connection management is not a command and is not
gated by it — an operator must be able to select and observe a device with
commands disabled, per [T-028](cards/T-028-usb-observer.md)'s "keep operator
commands disabled."

## 8. State machine

`Status.State` values, covering ADR 0006's named examples ("saved device
missing, port open awaiting telemetry, port access failure, vehicle reporting,
vehicle traffic interrupted") plus the states needed to make startup, ambiguity,
and handoff explicit:

| State | Meaning | Entered from |
|---|---|---|
| `DEVICE_MISSING` | A saved profile's device does not currently resolve in the inventory. | Startup; device removed before ever connecting. |
| `AMBIGUOUS` | More than one inventory device matches a profile's identity; none is chosen automatically ([ADR 0006](../adr/0006-operator-connection-readiness.md): "a different device occupying the saved port is not silently accepted"). | Startup; inventory refresh while `DEVICE_MISSING`. |
| `IDLE` | Device resolves; not connected. Operator has not pressed connect, or explicitly disconnected. | Default; `RELEASED` + operator reconnect intent. |
| `OPENING` | Connect requested; awaiting the OS open() result. | `IDLE`, `DEVICE_LOST` (auto-retry, §9). |
| `ACCESS_FAILED` | Open failed. `DetailedError` carries only what the OS/library reported (permission denied, already open by another process) — never an inferred cause. | `OPENING`. |
| `OPEN_AWAITING_TRAFFIC` | Port open; no valid MAVLink parsed yet. Ordinary for a ground radio attached before the aircraft powers on ([ADR 0006](../adr/0006-operator-connection-readiness.md)). | `OPENING`. |
| `REPORTING` | Port open, valid MAVLink received within the last 60 seconds. A heartbeat is not required. | `OPEN_AWAITING_TRAFFIC`; `INTERRUPTED` on recovery. |
| `INTERRUPTED` | Was `REPORTING`; no valid MAVLink frame for more than 60 seconds, but the device/port is still present. Distinct from `DEVICE_LOST` — telemetry silence is never treated as evidence the device was removed. | `REPORTING`. |
| `DEVICE_LOST` | The OS reports the device gone (unplugged) while connected. | `OPENING`, `OPEN_AWAITING_TRAFFIC`, `REPORTING`, `INTERRUPTED`. |
| `TRANSPORT_FAILED` | Read/write or event-stream failure without evidence of removal. The observed error is retained; no cause is guessed. | Any open state. |
| `RELEASED` | Explicit operator disconnect (MissionPlanner handoff). Retries are cancelled; the connection stays here until `Connect` is called again. | Any connected state, via `Manager.Disconnect`. |

`AMBIGUOUS` and `DEVICE_MISSING` require operator selection, matching
[ADR 0006](../adr/0006-operator-connection-readiness.md)'s "ambiguous identity
requires selection" — `Manager.Connect` never auto-resolves either from
inside these states.

## 9. Startup policy and retry

**Decision, settled here, resolving ADR 0006's open item** ("remembering
settings and deciding whether to connect on a new application launch are
distinct behaviors; the latter remains to be specified"): on backend start,
`Manager` auto-connects every saved profile that was in a connected state
(anything but `RELEASED`) when the backend last stopped, and whose device
resolves unambiguously. A profile the operator explicitly disconnected before
shutdown stays `RELEASED` after restart and is never auto-connected — restart
must not silently undo an intentional handoff.

Bounded retry applies to `DEVICE_LOST` and `ACCESS_FAILED` only, not to
`RELEASED`. It follows the same exponential-backoff-to-ceiling shape the
frontend already uses for its own SSE reconnect (`frontend/src/stream/live.ts`,
`RETRY_MIN_MS`/`RETRY_MAX_MS`): starting at 2s, doubling, capped at 30s, reset
on the next successful `OPENING`. These starting bounds are proposed, not
measured; [T-049](cards/T-049-connection-recovery.md) measures actual recovery
latency on hardware and may revise them. `Disconnect` always cancels a pending
retry immediately, matching [ADR 0006](../adr/0006-operator-connection-readiness.md):
"an intentional disconnect stops automatic reconnection."

Reboot detection adds no new signal at the connection layer. A full
vehicle-loss-then-rediscovery cycle is already `vehicle.Fold`/`Expire`'s
existing `HeartbeatTTL` behavior (`FleetEventType.VEHICLE_LOST` then
`VEHICLE_DISCOVERED`); the connection layer must not invent a second,
device-level reboot signal alongside it. Retained snapshots (mission,
parameters) are not revalidated by a connection state change alone — only by
new addressed traffic — so a snapshot fetched before a `VEHICLE_LOST` stays
labeled against that boundary until the owning feature (`T-029`, mission
display) re-fetches it. That labeling is a display concern; this contract only
guarantees the connection layer never manufactures a false "verified" signal
for it.

## 10. Journey mapping

| ADR 0006 journey | This contract's mechanism |
|---|---|
| First launch, no simulator | `Inventory.List` before any `Connect`; empty fleet renders from `Manager.Connections()` returning `[]` or all-`IDLE`, not an error. |
| Bench observation | Operator picks a `Device` from `/api/connections/devices`, `POST .../connect` with `Settings{BaudRate:...}`; `OPEN_AWAITING_TRAFFIC` -> `REPORTING` without a battery, since HEARTBEAT needs no flight battery. |
| Field observation | Ground radio attached before the aircraft: `OPEN_AWAITING_TRAFFIC` is the expected resting state, not an error. |
| Tool handoff (MissionPlanner) | `POST /api/connections/disconnect` -> `RELEASED`; port is OS-closed, so MissionPlanner can open it; reconnecting needs no application restart. |
| Interruption and return | Ground device removal -> `DEVICE_LOST`; aircraft silence alone -> `INTERRUPTED`; both distinguished per §8 and never conflated. |

## 11. Error taxonomy

Every surfaced error names only what was actually observed:

- `ACCESS_FAILED.DetailedError` — the OS/library error string (permission
  denied, device or resource busy). Never a guessed baud rate or "wrong
  device" — [ADR 0006](../adr/0006-operator-connection-readiness.md): "silence
  alone cannot establish a wrong device, incorrect baud rate, powered-off
  aircraft or failed radio path."
- `200 OK` on repeated identical `POST .../connect` returns the existing status
  without another open. `409 Conflict` means an already-open device has different
  requested settings, or the OS reports busy. The response distinguishes these
  cases without naming an unobserved owner.
- `404 Not Found` on `POST .../connect` — the `device_id` from a stale
  inventory snapshot no longer resolves; the caller must re-list.
- Silent ports are never escalated to an error. `OPEN_AWAITING_TRAFFIC` is a
  first-class rest state, not a timeout-to-failure.

## 12. Open questions carried forward

Not settled by this document; each belongs to the card named:

- Exact serial device-access library and OS permission handling —
  [T-046](cards/T-046-serial-acquisition.md).
- Profile storage schema, location and identity-matching algorithm — resolved
  by [T-047](cards/T-047-saved-connections.md)/[ADR 0010](../adr/0010-connection-profile-persistence.md).
  A hardware-backed PTY restart harness for profiles, extending T-046's, was
  scoped out of T-047 and remains unexecuted.
- Measured retry/interruption timing bounds (§9's numbers are proposed
  starting points) — [T-049](cards/T-049-connection-recovery.md).
- Packaging, launcher, and the final supported OS list (§2's assumption) —
  [T-050](cards/T-050-local-observer-launch.md).
- Radio diagnostics available from the actual LR900 pair — remains
  unverified per [ADR 0006](../adr/0006-operator-connection-readiness.md);
  this contract assumes none and adds no diagnostic message.

## Verification

Reviewed against [ADR 0006](../adr/0006-operator-connection-readiness.md) and
the current transport (`internal/codec/frame.go`, `internal/bridge/bridge.go`,
`internal/routes/table.go`) and stream (`internal/stream/hub.go`,
`internal/stream/sse.go`, `frontend/src/stream/events.ts`,
`frontend/src/stream/live.ts`) code cited throughout. Every ADR 0006 journey
in §10 walks through a normal and at least one failure path. No code changes
accompany this document; `./scripts/kanban check` validates the linked cards.

## T-046 wire and implementation notes

Status uses snake-case JSON fields: `id`, `device`, `settings`, `state`,
`opened_at_ms`, `last_frame_at_ms`, `vehicle_keys`, and optional
`detailed_error`/`error_code`. Settings are `{"baud_rate":57600}` (8N1); keys
are `{"system_id":1,"component_id":1}`. Baud is a positive integer; the
library/OS decides whether the requested speed is supported. Inventory includes
available OS metadata (`description`, `serial_number`, `manufacturer`, `vid`,
`pid`), never an inferred radio identity. Device IDs are path-based snapshot
identities; persistent matching remains T-047.

`vehicle_keys` reflects last-source attribution within the existing 60-second
route TTL, not heartbeat liveness. Moving an identity to another connection or
closing its link removes the old attribution. Timestamp values are Unix
milliseconds; RELEASED preserves historical timestamps without claiming freshness.
`acquisition` publishes status/frame evidence and retains the latest serialized
snapshot per connection. Existing frontend consumers can ignore the new event;
frontend consumption and controls belong to T-048.

Connect bodies are bounded, strict JSON with the existing same-origin guards.
Permission-denied opens return 403, missing devices 404, busy/settings conflicts
409, invalid settings 400, and unclassified transport failures 500 with observed
details. No status guesses baud mismatch or another application's identity.
Inventory refresh runs once per second while the manager lives; an enumeration
error alone never marks a device lost. A closed port with a still-present device
is TRANSPORT_FAILED, not DEVICE_LOST. Explicit reconnect is supported; no
background reconnect or profile startup behavior is implemented in T-046.

## T-047 wire and implementation notes

Saved profiles live in `internal/connection.Profile`/`Store`, persisted by
`FileStore` as one versioned JSON document (default
`os.UserConfigDir()/yalb-gcs/connection-profiles.json`, overridable with
`GCS_CONNECTION_PROFILES_PATH`) replaced atomically on every write. Full
rationale, the identity-matching algorithm and the CONNECTED/IDLE/RELEASED
intent semantics are recorded in
[ADR 0010](../adr/0010-connection-profile-persistence.md); this section is
only the wire/behavior summary.

`GET /api/connections` includes one entry per saved profile from backend
startup, keyed by the profile's own id rather than a `Device.ID` — this is
what makes a saved connection's identity stable across a restart that renames
its OS port. `POST /api/connections/connect` accepts `profile_id` as an
alternative to `device_id`; passing both is the explicit-selection path used
to resolve `AMBIGUOUS`/`DEVICE_MISSING`, and the chosen device is never
written back into the profile's saved identity. `POST /api/connections/profiles`'s
`device_id` is always re-resolved against current inventory, never trusted
from the request body.

`AMBIGUOUS` is a new `Status.State` alongside T-046's ten. It and
`DEVICE_MISSING` are only evaluated at backend startup and on each
`POST /api/connections/connect` call in this task; there is no
background/sweep-driven re-evaluation of an already-seeded `DEVICE_MISSING`
or `AMBIGUOUS` entry; T-049 owns that. A profile lacking a full
serial-number/VID/PID identity never auto-matches and always requires an
explicit `device_id`, which is expected for adapters that report no serial
number, not an error condition.

The T-047 acceptance suite (atomic save/reload/delete, invalid/version-
mismatched/truncated files, an injected write failure that must not replace
good data, concurrent writers that must never corrupt the file, startup
resolution, and HTTP CRUD/connect/disconnect) is executed by
`go test -race ./internal/connection/...`; see the T-047 card's Notes for the
full recorded command list and platform results. A real-backend PTY hardware
harness for profile save/reload/restart, extending T-046's, was scoped out of
this pass and remains unexecuted — no hardware or PTY evidence is claimed for
T-047.

The fixed, startup-configured UDP target appears in inventory/status and can be
released/reopened through its existing ID; callers cannot introduce arbitrary
UDP endpoints. Serial-only startup still runs the bridge, mission coordinator,
recording sink and SSE hub. Connection management is available with operator
commands disabled. Observer-originated traffic remains heartbeat, addressed
message-interval requests, and explicitly requested mission download traffic.
