# Task 05 — Thin HTTP application boundary

**Outcome:** the web UI can discover and invoke Go calculations and workflows.
Dependency: [04](04-workflow-engine.md).

## Work

- Expose the book-method mappings and documented adaptations from the core as
  specified in the [handoff](README.md); preserve them in discovery and traces.
- Define the API schema from the implemented core and tests at this task's start.
  Cover equation/pattern discovery, evaluation and structured issues/traces.
  Keep HTTP parsing, status codes and serialization outside the calculator package.
- Choose one contract authority and generate/check frontend boundary types where
  appropriate. TypeScript types alone do not validate untrusted response data.
- Reject malformed, missing, extra or incompatible request fields as specified
  by the contract. Put reasonable bounds on payloads and batch requests.
- Keep cancellation and request identity explicit so an older result cannot be
  mistaken for a newer candidate. Carry Task 04's evaluation identity across the
  boundary independently of undoable design revisions. Do not add goroutines
  inside scalar formulas.
- Make local frontend/backend startup straightforward and separate from GCS.
  Deployment needs a Go service as well as static assets; a static-only host does
  not execute the calculation core.

## Acceptance checks

- HTTP integration tests cover success, bad JSON, unknown mode/equation, invalid
  numeric data, oversized input, and structured domain failure.
- Core calls and API calls return equivalent physical results, provenance and
  requirement states for the same fixture.
- API code has no independent equations or solver decisions.
- The executable starts and serves the documented discovery/evaluation path.
- API contracts can be reused by a future MCP adapter without making the core
  depend on HTTP. MCP need not make loopback HTTP calls to use the same package.
