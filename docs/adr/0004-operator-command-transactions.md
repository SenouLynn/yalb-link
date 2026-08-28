# ADR 0004: Guarded operator command transactions

## Decision

Expose only `POST /api/commands/arm` and `POST /api/commands/arm/resolve`,
gated by `GCS_COMMANDS_ENABLED`. The arm request sends
`MAV_CMD_COMPONENT_ARM_DISARM` to component 1 with force-arm parameter 2 fixed
at zero, then synchronously returns an immutable transaction snapshot. There is
no generic command dispatcher and no generic resolution surface.

An eligible `COMMAND_ACK` must match all three identities: its frame sender is
the commanded vehicle `(system,1)`, its command is 400, and its payload target
is this GCS `(255,190)`. MAVLink 2 target extension fields left at zero fail
closed. `MAV_RESULT_IN_PROGRESS` is non-terminal.

## Ambiguity policy

`COMMAND_ACK` identifies a command number, not an invocation. After request A
times out, a late or duplicated ACK for A is indistinguishable from an ACK for
a later request B. Therefore timeout, cancellation, and delivery-uncertain
write failure poison that vehicle/command key. MAVLink traffic never clears a
poison: recovery traffic and a late ACK cannot prove which invocation answered.
No retry is attempted.

An unknown link fails before transport write, so it publishes nothing and does
not poison. Any other write error may follow a partial delivery; it publishes
one `SEND_FAILED` snapshot and poisons.

The poisoned key retains its terminal transaction, and an arm request refused
because of it answers `409` with that snapshot under code `command_unresolved`.
The workflow therefore survives a page reload: the operator is told which
transaction to resolve rather than being left to remember it.

## Operator attestation and quarantine

`POST /api/commands/arm/resolve` clears one poison. The operator states the
armed state they observed, and the attestation is recorded beside the
transaction — the terminal state is never rewritten, because the ambiguity
remains a historical fact that no later evidence can undo. The observed state
is an enum on the wire and in the contract; a missing, empty, or unknown value
is rejected, so an attestation nobody made can never read as one they did.

A resolution reference is `(registry_epoch, id)`. Transaction ids are a
per-process counter, so the epoch stops a client that survived a restart from
resolving an unrelated transaction that reused the number.

Clearing a poison reopens what the poison was guarding: with the key
commandable again, a late ACK for the resolved invocation is once more
indistinguishable from the answer to the next one. Resolution therefore moves
the key into a quarantine — `DefaultResolutionQuarantine`, 30 seconds — during
which commands are refused with code `command_quarantined` and a relative
`retry_after_ms`. The duration is its own constant rather than a multiple of
the command timeout: late-ACK latency is a property of the link, so deriving it
would let a tuned timeout silently shrink the protection.

This deliberately relaxes the invariant this ADR previously held. Ambiguity
used to persist until process restart; it now persists until an operator
attests, after which a stated 30-second bound stands in for a guarantee. An ACK
delayed beyond the window can still collide with a later command. That residual
risk is accepted and recorded rather than engineered away, because MAVLink
offers no invocation identity with which to remove it.

The quarantine bounds in-process attestation recovery only. Restart remains the
other recovery boundary and clears every in-memory protection — poison,
quarantine, and the epoch alike.

Resolution publishes its snapshot outside the registry mutex and does not roll
back if publication fails. No production publisher can report failure, and a
rollback would restore a poison that subscribers had already been told was
resolved. Recording is bounded and asynchronous and may drop an event after
accepting it, so attestations reach the normal recording pipeline on the same
best-effort terms as every other snapshot. A guaranteed audit record would be a
separate persistence decision.

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

Both handlers require JSON, bound and strictly decode the body, reject
cross-origin and cross-site browser requests, and rely on the development proxy
preserving the original Host. These are local-development CSRF defenses, not
operator authentication or authorization.

Transactions and attestations carry the fixed label `local-operator`. It
records that a human acted, not who: nothing on this surface authenticates a
caller. A later authentication or TAK integration can supply a real principal
at the HTTP boundary without changing command semantics.
