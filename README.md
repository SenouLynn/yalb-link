# YALB-GCS
Exploring GCS build-out, UAV fleet management, and multi-protocol/multi-node telemetry fusion. 

Natural successor to `flight-hud-trajectory`, a read-only MAVLink sandbox I had built incrementally starting with basic flight instruments. 

## Key High-Level Considerations
- **UAV Targeted**: Feature families initially targetting Ardupilot-supported UAV's. 
    - **MAVLink**: Core messaging schema driving architecture decisions. Re-implementing pieces of Mission Planner and QGroundControl. 
    - **Telemetry**: Track & replay UAV and sensor state
    - **Mission Mgmt**: Track & manage UAV guidance state
    - **Configuration**: In-suite parameter management
    - **Video**: Protocol agnostic, digital/analog streams both considered
- **Multi-Protocol**: Telemetry and signal ingestion should extend to non UAV classes of streams.
    - **Lora**: Meshtastic can be used by individual operators, static sensors, sensor groups, or telemetry relays. 
    - **Digital FPV**: OpenIPC rigs, or a handrolled digital link, is a strong candidate for dedicated or redundant video streams. 
- **Composability**: User-facing layer and associated pipelines should be granular and composable. The core feed should have maximum trust before consumed by components.

## Key Development Considerations
- **Human in the Loop**: Yes, agents will help be doing most of the mechanical heavy-lifting. Humans will be the sole arbiter of architecture decisions and final code inclusion. Nothing goes onto the main branch without both adversarial agentic review AND human review. 
- **Self Documenting**: ADR's, changelogs, and runbooks, and commit history should contain enough detail for any human or agent to quickly familiarize themselves with broad directionality, current state, and work that may be in flight. 
- **Maximal Strictness**: LSP's, type-checkers, linters, and basic syntax and pre-compile tools are the cheapest feedback loops available. It's easier to loosen configs later than it is to tighten them, start strict and unwind only if adequately reasoned against. 
- **Portability**: To enable SITL, various targets, and testing rigs, previous iterations have gotten alot of juice building structure hexagonally. Ports + adapters have strong case keeping functionality modular. We can isolate and build trust for core edges of the system by hitting them from different angles. 
- **Environments**: Production and dev environments should be identical. We build trust by testing functionality as close to production as we can. SITL, for example, allows us to test fleet-view with real fake copter and plane Ardupilot instances at once before field tests. 
- **Testing**: Generally TDD has worked for agents, not humans really. Establish trust early and incrementally, with idempotency. Test data-flows, test the *correct* layer, and with appropriate granularity. 

