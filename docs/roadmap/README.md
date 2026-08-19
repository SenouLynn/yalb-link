# Roadmap

Build order, tier breakdowns, and porting reference for yalb-gcs.

**Build sequence: 0 → 3 → (1 ∥ 2) → 4 → 5 → 6 → 7 → 8 → 9 → 10.** Tier numbers are
identifiers, not queue positions — codegen runs right after contracts so Tier 1 has
real generated types. Each tier file states its own gate.

Every exit gate is a `make` target (`make gate-tier-0` … `gate-tier-4`).

## Index

| File | Purpose |
|---|---|
| [order-of-operations.md](order-of-operations.md) | Canonical build sequence — tier summary, exit gates, resolved decisions, out-of-scope table. Start here. |
| [tier-0-3-adversarial-review.md](tier-0-3-adversarial-review.md) | Adversarial review of Tiers 0–3 — verified defects, ordering change, Tier 0 additions. Read before starting Tier 0. |
| [port-plan.md](port-plan.md) | What to port from flight-path-hud and where it lands. Canonical for **algorithms and file mapping only** — build tooling, codegen, and sequencing statements in it are superseded by order-of-operations and the tier files. |
| [tier-0-proto-contracts.md](tier-0-proto-contracts.md) | **[1st]** Contract decisions that are free now and breaking later: identity, telemetry/transaction split, streaming limits, role annotations, go_package, lint posture |
| [tier-1-pure-domain-logic.md](tier-1-pure-domain-logic.md) | **[3rd ∥]** Go codec dispatch table + TS resolvers, no sockets, no Redis |
| [tier-2-parity-apparatus.md](tier-2-parity-apparatus.md) | **[3rd ∥]** Capability matrix derived from the dispatch table, golden byte fixtures, test harness |
| [tier-3-codegen.md](tier-3-codegen.md) | **[2nd]** buf.gen.yaml, generated Go + TS stubs, CI gates |
| [tier-4-bridge-core.md](tier-4-bridge-core.md) | Vehicle fold, route table, Docker Compose, SITL image. **Built** — see its As Built section |
| [tier-5-transport-live.md](tier-5-transport-live.md) | UDP transport, Redis client, GCS heartbeat, **discovery request round trip (ch.7, new)**, first SITL connection. **Chapters 1–3 built** — see its As Built section. The stream-rate question is closed by ADR-0010 |
| [tier-6-connect-services.md](tier-6-connect-services.md) | FleetService, TelemetryService, track layer, TelemetryLog |
| [tier-7-read-transactions.md](tier-7-read-transactions.md) | Parameter read/list, mission download, ParametersPanel |
| [tier-8-write-transactions.md](tier-8-write-transactions.md) | Command registry, param write, mission upload, arm/disarm, guided |
| [tier-9-protocol-adapters.md](tier-9-protocol-adapters.md) | ADS-B and Meshtastic adapters, Track proto normalization |
| [tier-10-ui-composition.md](tier-10-ui-composition.md) | Instrument tier, composed layouts, full HUD |

Order of authority: **ADRs → order-of-operations.md → tier files → port-plan.md.**
Where this roadmap conflicts with an ADR, the ADR wins.
