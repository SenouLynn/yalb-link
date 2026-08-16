# Roadmap

Build order, tier breakdowns, and porting reference for ligma-gcs.

## Index

| File | Purpose |
|---|---|
| [order-of-operations.md](order-of-operations.md) | Canonical build sequence — tier summary, exit gates, resolved decisions, out-of-scope table. Start here. |
| [port-plan.md](port-plan.md) | What to port from flight-path-hud, where it lands, and architecture mapping. Canonical reference for existing logic. |
| [tier-0-proto-contracts.md](tier-0-proto-contracts.md) | Proto audit, force field removal, buf lint gate |
| [tier-1-pure-domain-logic.md](tier-1-pure-domain-logic.md) | Go codec + TS resolvers, no sockets, no Redis |
| [tier-2-parity-apparatus.md](tier-2-parity-apparatus.md) | Capability matrix, golden byte fixtures, test harness |
| [tier-3-codegen.md](tier-3-codegen.md) | buf.gen.yaml, generated Go + TS stubs, CI gates |
| [tier-4-bridge-core.md](tier-4-bridge-core.md) | Vehicle fold, route table, Docker Compose, SITL image |
| [tier-5-transport-live.md](tier-5-transport-live.md) | UDP transport, Redis client, GCS heartbeat, first SITL connection |
| [tier-6-connect-services.md](tier-6-connect-services.md) | FleetService, TelemetryService, track layer, TelemetryLog |
| [tier-7-read-transactions.md](tier-7-read-transactions.md) | Parameter read/list, mission download, ParametersPanel |
| [tier-8-write-transactions.md](tier-8-write-transactions.md) | Command registry, param write, mission upload, arm/disarm, guided |
| [tier-9-protocol-adapters.md](tier-9-protocol-adapters.md) | ADS-B and Meshtastic adapters, Track proto normalization |
| [tier-10-ui-composition.md](tier-10-ui-composition.md) | Instrument tier, composed layouts, full HUD |

ADRs are law. Where this roadmap conflicts with an ADR, the ADR wins.
