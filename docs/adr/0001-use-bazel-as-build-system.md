# 0001 — Use Bazel as the Build System

**Status:** Accepted

## Context

This repo is expected to grow across multiple languages over time. Key constraints shaping the build system decision:

- **Polyglot from the start** — language boundaries should not require separate build toolchains or CI pipelines
- **Agentic workflows** — AI agents will participate in development, including maintaining build boilerplate; the build system must be structured and declarative enough for agents to reason about and patch reliably
- **Determinism and isolation at checkpoints** — specific moments (tests, releases, artifact production) must be hermetic and idempotent, with no implicit dependency on ambient environment state
- **Repo health as a feedback loop** — tests (and feedback) is first-class; the build system should make running tests cheap, cacheable, and trustworthy so they can be used as a reliable signal by both humans and agents

## Decision

Use **Bazel** with **Bzlmod** (`MODULE.bazel`) as the single build system for all languages in this repository.

## Consequences

**Accepted costs:**
- Higher upfront cost to describe targets explicitly vs. convention-based tools. This can be confusion if not familiar with indiosynchracies. 
- Language support quality varies; some rulesets are less mature than native toolchains

**Expected benefits:**
- Single `bazel test //...` is a trusted, hermetic signal regardless of language mix
- Incremental builds are correct by construction, not by convention
- Agent-maintained BUILD files are feasible. The format is structured, local, and deterministic
- Cache sharing across machines and CI comes for free
