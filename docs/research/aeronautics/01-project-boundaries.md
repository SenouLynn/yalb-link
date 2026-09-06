# Task 01 — Establish the standalone project

**Outcome:** a separately runnable calculator project inside yalb-link with a Go
domain boundary and its own Vite/React frontend. No GCS frontend reuse or dependency.

Read [the handoff](README.md) first. Dependency: none.

## Work

- Choose a clear project directory, provisionally `aeronautics/`, with a separate
  Go module and frontend package. Finalize names while implementing; the layout
  is a proposal, not an existing contract.
- Establish a transport-free Go calculator package, a thin application entry
  point, and a separate web directory. React owns presentation and draft text;
  Go owns physical values, workflow decisions and numeric results.
- Set strict Go/TypeScript checks and independent dependency lock/build files.
  Do not couple calculator startup to MAVLink, SITL, Redis, SQLite or GCS services.
- Document local run/test/build commands that have actually been executed.
  Confirm how the separate module interacts with repository discovery/build
  tooling rather than silently modifying the GCS gates.
- Keep the frontend shell minimal. Do not invent calculation endpoints or put
  placeholder physics in TypeScript before Tasks 02–05.

## Acceptance checks

- Go package checks and frontend typecheck/build run independently.
- Existing GCS sources and dependencies are unchanged by the new module.
- A package dependency inspection confirms the core has no HTTP, MCP, UI, storage
  or live-telemetry dependencies.
- README distinguishes implemented shell behavior from future tasks.

**Stop after the boundary exists.** Equation models belong to Task 02; no generic
solver, worker pool, plugin system, or MCP server is required here.
