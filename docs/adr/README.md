# Architecture Decision Records

Numbered Markdown documents capturing significant architectural choices and their
rationale. Name files `NNNN-short-title.md`.

**ADRs are law.** Where the roadmap or a tier file contradicts an ADR, the ADR wins.
Where an ADR contradicts an executable artifact about what the system *does*, see
ADR-0008 §3 — the artifact wins and the ADR is amended.

**How ADRs change is itself an ADR: [0008](0008-documentation-lifecycle-and-amendments.md).**
In short — the Context and Decision of an Accepted ADR are immutable; new facts that do
not change the decision go in a dated `## Amendment (YYYY-MM-DD) — <scope>` block naming
what was wrong, what still stands, and the consequence; a changed decision means a new ADR
with `Supersedes: NNNN`, never an edit; and a fourth amendment is the signal to supersede
rather than amend again.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-use-bazel-as-build-system.md) | Bazel as the build system | Accepted |
| [0002](0002-stack-go-react-websocket-docker.md) | Go / React / Connect / Docker stack | Accepted |
| [0003](0003-frontend-adapter-pattern-and-sitl-transport.md) | Frontend adapter pattern and SITL transport | Accepted |
| [0004](0004-multiprotocol-track-layer.md) | Multi-protocol track layer | Accepted |
| [0005](0005-security-and-auth-model.md) | Security and auth model | Accepted |
| [0006](0006-toolchain-version-coupling.md) | Toolchain version coupling and dependency risk | Accepted, amended ×1 |
| [0007](0007-firmware-variance-via-capability-negotiation.md) | Firmware variance via capability negotiation | Accepted, amended ×1, extended by 0009 |
| [0008](0008-documentation-lifecycle-and-amendments.md) | Documentation lifecycle and amendments | Accepted |
| [0009](0009-parameter-acquisition-paradigms.md) | Parameter acquisition paradigms | Accepted |
