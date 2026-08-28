# ADR 0004: Guarded operator command transactions

## Decision

Expose only `POST /api/commands/arm`, gated by `GCS_COMMANDS_ENABLED`. The
request sends `MAV_CMD_COMPONENT_ARM_DISARM` to component 1 with force-arm
parameter 2 fixed at zero, then synchronously returns an immutable transaction
snapshot.

An eligible `COMMAND_ACK` must match all three identities: its frame sender is
the commanded vehicle `(system,1)`, its command is 400, and its payload target
is this GCS `(255,190)`. MAVLink 2 target extension fields left at zero fail
closed. `MAV_RESULT_IN_PROGRESS` is non-terminal.

## Ambiguity policy

`COMMAND_ACK` identifies a command number, not an invocation. After request A
times out, a late or duplicated ACK for A is indistinguishable from an ACK for
a later request B. Therefore timeout, cancellation, and delivery-uncertain
write failure poison that vehicle/command key until process restart. Recovery
traffic and a late ACK cannot safely clear it. No retry is attempted.

An unknown link fails before transport write, so it publishes nothing and does
not poison. Any other write error may follow a partial delivery; it publishes
one `SEND_FAILED` snapshot and poisons.

## Ordering and publication

The registry registers a buffered ACK waiter before writing. After a successful
write it publishes `PENDING`, then waits and publishes a terminal clone. This
keeps pending-before-terminal ordering even if the ACK races the write. The ACK
sink only validates and delivers; it never publishes and always returns nil.

The registry publishes through a fan-out that excludes itself, and never while
holding its mutex. Command events reach current SSE subscribers and recording,
but the hub does not retain or bootstrap them because a transaction is history,
not current vehicle state. Recording is the durable history.

Client cancellation differs from timeout: it says the requester disappeared,
not that the vehicle failed to answer. Since the client is already gone, its
`CANCELLED` response may not be observable there, but the terminal event is
still published and recorded.

## Security scope

The handler requires JSON, bounds and strictly decodes the body, rejects
cross-origin and cross-site browser requests, and relies on the development
proxy preserving the original Host. These are local-development CSRF defenses,
not operator authentication or authorization.
