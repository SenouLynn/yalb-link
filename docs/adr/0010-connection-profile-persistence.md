# ADR 0010: Connection profile persistence and identity matching

Date: 2026-09-23

Status: Accepted — implementation landed alongside this record.

## Context

[T-047](../tasks/cards/T-047-saved-connections.md), required by
[ADR 0006](0006-operator-connection-readiness.md) and scoped by
[the connection contract](../tasks/connection-contract.md), needs saved
connection profiles ("Bench controller", "Field radio") to survive backend
and browser restarts, resolve to the correct physical device even when its
OS port name changes, and never silently accept a different device occupying
a previously used port. The contract left profile storage schema, the exact
identity-matching algorithm, and the CONNECTED/IDLE/RELEASED intent semantics
open for this task to settle. These choices are costly to reverse once
profiles exist on operators' machines (schema) or once the frontend (T-048)
starts depending on the HTTP surface, so they are recorded here rather than
only in code comments.

## Decision

**Storage.** One versioned JSON document (`{"version":1,"profiles":[...]}`)
under the OS per-user configuration directory
(`os.UserConfigDir()/yalb-gcs/connection-profiles.json`), overridable with
`GCS_CONNECTION_PROFILES_PATH`. Every write goes through
`internal/connection.FileStore`, which writes a temp file in the same
directory, `fsync`s it, and `os.Rename`s it into place, so a reader never
observes a partially written document and a failed write never touches the
previous good file. A version field is a hard gate: any value other than the
currently understood version is treated as unreadable, not guessed at or
migrated — there is no migration path yet, and inventing one speculatively is
worse than failing loudly.

**Concurrency scope.** `FileStore` serializes its own read-modify-write
cycle with a single process-local mutex. It does not coordinate with a
second OS process writing the same path; atomic replace still guarantees the
file itself is never torn by two writers, but a genuinely concurrent second
writer can overwrite the first's change (last write wins). This is accepted
because the connection service is scoped to one local observer process
(T-050); cross-process file locking is not implemented.

**Identity matching.** A profile auto-resolves to an inventory device only
when its saved `SerialNumber`, `VID` and `PID` are all present and match
exactly one currently enumerated serial device. Zero matches is
`DEVICE_MISSING`; more than one is the new `AMBIGUOUS` state; a profile
missing any of the three identity fields (a cheap adapter with no exposed
serial number) never auto-matches at all. In every one of these cases the
operator must connect with an explicit `device_id` (`POST
/api/connections/connect {profile_id, device_id, settings}`), which is used
verbatim and never written back into the profile's saved identity — ADR
0006's "do not silently substitute another device" applies to the matcher
itself, not just to its failure modes.

**Startup policy.** On construction, `Manager` loads every saved profile and
seeds `Connections()` with one entry per profile, keyed by the profile's own
ID (not a `Device.ID`, which is not stable across restarts). A profile whose
persisted `Intent` is `CONNECTED` is auto-opened if its identity resolves to
exactly one device; otherwise it is seeded `DEVICE_MISSING` or `AMBIGUOUS`.
`Intent` of `IDLE` (saved, never yet connected) seeds `IDLE`; `RELEASED`
(explicit prior disconnect) seeds `RELEASED` and is never auto-connected —
this is what makes "remembering a profile does not itself override
intentional disconnect" (T-047 acceptance) hold. `Intent` is written through
on every `Connect`/`Disconnect` against a profile-backed connection, decoupled
from the fine-grained transient `Status.State` machine (`OPENING`,
`REPORTING`, `INTERRUPTED`, …) — only these three values are ever persisted.

**Persistence failures reject mutations, not the physical release.** Saving,
deleting, or marking a profile `CONNECTED` all fail the calling request
outright if the underlying write fails, and in the connect case this check
happens *before* the device is opened, so acquisition is never left running
un-recorded. Disconnect is the one asymmetric case: the OS port is always
released even if persisting the resulting `RELEASED` intent fails, because an
operator handing a port to MissionPlanner must not be blocked by a disk
error; the failure is still returned to the caller so it is reported, not
swallowed. An unreadable profile store at startup (corrupt file, unsupported
version) disables automatic acquisition only — manual, non-profile
`Connect`/`Disconnect` by device id keeps working, and `GET
/api/connections/profiles` surfaces the same read error so the condition is
actionable rather than silently empty.

**Non-goals, deferred deliberately.** Bounded retry/backoff for
`DEVICE_LOST`/`ACCESS_FAILED` and any UI are explicitly out of this task's
scope per its own card ("UI controls and retry after startup remain
T-048/T-049") and are not implemented here: a profile that becomes
`DEVICE_MISSING` or `AMBIGUOUS` after startup does not retry on its own:
another `Connect`/`ConnectProfile` call re-resolves it. Ongoing (post-startup)
promotion of `DEVICE_MISSING` to `AMBIGUOUS` as inventory changes is also
left to T-049, not implemented in the periodic sweep.

## Consequences

- Profiles are portable data (plain JSON), inspectable and editable by hand
  in a pinch, at the cost of no built-in encryption — acceptable since the
  stored fields (name, device path/identity, baud rate) are not secrets.
- The one-process-owns-the-file assumption means running two backend
  instances against the same profiles path is unsupported; this matches
  every other piece of local backend state (recording DB, HTTP bind) and is
  not a new constraint.
- Because connection IDs for profile-backed connections are profile IDs, not
  device IDs, a UI (T-048) can hold a stable reference to a saved connection
  across restarts and port renames without re-resolving anything itself.
- `AMBIGUOUS`/`DEVICE_MISSING` recovery is deliberately inert without an
  explicit API call in this task; T-049 is expected to add bounded retry and
  a sweep-driven re-evaluation on top of the same `Store`/`matchIdentity`
  primitives introduced here, not a replacement for them.

## Verification point

`go test -race ./internal/connection/... ./internal/bridge/... ./internal/stream/... ./cmd/gcs/...`,
native macOS (`CGO_ENABLED=1`) and Linux production (`CGO_ENABLED=0
GOOS=linux GOARCH=arm64`) builds, `go vet ./...` and `./scripts/kanban check`
all pass (recorded on the T-047 card). Coverage includes atomic save/reload/
delete, invalid/version-mismatched/truncated files, an injected write failure
that must not replace good data, concurrent writers that must never corrupt
the file, startup resolution across matched/missing/ambiguous/idle/released
profiles, connect/disconnect intent persistence, and HTTP CRUD plus
connect/disconnect by `profile_id` with the same-origin guards operator
commands stay independent of. A real-backend PTY hardware harness exercising
save/reload/restart against an actual serial adapter, extending T-046's
harness, was scoped out of this pass and remains unexecuted; the T-047 card
records this explicitly rather than implying hardware acceptance occurred.
