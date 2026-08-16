# 0006 — Toolchain Version Coupling and Dependency Risk

**Status:** Accepted

## Context

ADR-0001 chose Bazel for determinism and hermeticity. Bringing that from an
aspiration to a working `bazel test //...` across Go and TypeScript surfaced a
class of risk the earlier ADRs did not name: **our pinned versions are not
independent of each other.** Bumping one forces bumps in three others, and every
one of these failures was silent, misleading, or both — the error message pointed
somewhere other than the cause.

This is not hypothetical. Every row below is something that actually broke.

### The chain, as encountered

Wiring Bazel meant a five-step forced upgrade, each step revealed only by
attempting the previous one:

1. `bazel run //:gazelle` failed with `go: unknown GOEXPERIMENT coverageredesign`.
   rules_go 0.52.0 passes that experiment to the Go toolchain; Go 1.25 removed it.
   The stdlib build died. Nothing in the error names rules_go.
2. Fixing that meant rules_go 0.52.0 → 0.62.0, which required gazelle 0.40.0 →
   0.52.2 (gazelle depends on rules_go and the two move together).
3. Adding the frontend required `aspect_rules_js`, which declares
   `bazel_compatibility = [">=7.6.0"]`. Our `.bazelversion` was 7.4.1, so the
   ruleset was uninstallable until Bazel itself moved to 8.7.0.
4. Bazel 8 changed external repo path separators from `~` to `+`, invalidating the
   lockfile and any path that had been written down.
5. `ts_project` refused to build because `tsconfig.app.json` pointed
   `tsBuildInfoFile` into `node_modules/.tmp/` — a directory Bazel materialises from
   the lockfile and therefore cannot also accept build outputs into.

### Couplings now known to exist

| Coupling | Breaks as | Detected by |
|---|---|---|
| `rules_go` ↔ Go SDK version | `unknown GOEXPERIMENT`, stdlib build failure | `bazel build //...` |
| `gazelle` ↔ `rules_go` | Load errors in generated BUILD files | `bazel run //:gazelle` |
| `aspect_rules_js` ↔ Bazel version | `bazel_compatibility` rejection at module resolution | `bazel fetch` |
| `aspect_rules_ts` ↔ TypeScript version | `ts_version_from` mismatch with `package.json` | `bazel build //frontend:typecheck` |
| `npm_translate_lock` ↔ pnpm lockfile format | Lockfile version unsupported by the ruleset | `bazel fetch @npm//...` |
| `tsconfig` paths ↔ Bazel-owned `node_modules` | ts_project validation failure | `bazel build //frontend:typecheck` |
| `protobuf-es` major ↔ buf plugin set | v2 emits service descriptors itself; a `connectrpc/es` plugin entry is a v1-era config that produces dead files | `buf generate` output shape |
| `buf` remote plugins ↔ BSR availability | Codegen unavailable offline | `make proto-gen` |

Two further items are dependency risk of a different shape but belong on the same
list, because they are also things a version bump can silently introduce:

- **A second generator for the same package.** Gazelle by default emits
  `proto_library` + `go_proto_library` claiming the same `importpath` as the
  committed buf output. Two generators racing for one Go package, with deps
  resolving to labels that do not exist.
- **Telemetry arriving with a ruleset.** `aspect_rules_js` pulls in
  `aspect_tools_telemetry`, which announces that it will begin collecting usage
  data on the next invocation. A transitive dependency changed the repo's external
  network behaviour without anyone choosing it.

---

## Decision

### 1. One source of truth per ecosystem; Bazel reads it, never duplicates it

`go.mod` owns Go dependency versions. `pnpm-lock.yaml` owns JS dependency
versions. Bazel consumes both — `go_deps.from_file(go_mod = "//:go.mod")` and
`npm_translate_lock(pnpm_lock = "//frontend:pnpm-lock.yaml")` — and never restates
a version. A version written in two places is a version that will disagree.

The three things Bazel *does* own, because nothing else can: the Bazel version
(`.bazelversion`), the ruleset versions (`MODULE.bazel`), and the Go SDK version
(`go_sdk.download`).

### 2. Pin the toolchain, do not float it

`go_sdk.download(version = "1.25.0")`, not `go_sdk.host()`. A host SDK makes the
build depend on whichever Go happens to be installed, which is precisely the
implicit dependency on ambient environment state that ADR-0001 exists to remove.
The same applies to `.bazelversion` — bazelisk fetches the pinned version rather
than using whatever `bazel` resolves to on the machine.

**The pinned Go SDK version and `go.mod`'s Go version must move together, and
`rules_go` must move with them.** That is the coupling that started the chain.

### 3. Version bumps are a batch, planned as one

An upgrade to any of {Bazel, rules_go, gazelle, aspect_rules_js, aspect_rules_ts,
Go, TypeScript} is assumed to force the others until proven otherwise. Bump them in
one commit, run `make bazel-test`, and record what moved and why. Do not attempt a
single-ruleset bump and abandon it half-applied: the intermediate states do not
build, and a half-applied bump is indistinguishable from a broken repo.

Check `bazel_compatibility` in a ruleset's `MODULE.bazel` on the BCR *before*
adding it. It is one `curl` and it is the difference between a five-minute change
and an afternoon.

### 4. Gates must be canaried, not assumed

Every failure above was silent in some direction, and so are gates. Two examples
from wiring this, both of which passed while being useless:

- `bazel build //frontend:typecheck` succeeded with a deliberate type error in the
  tree. `ts_project` runs typechecking in a separate action whose outputs live in
  the `typecheck` output group; building the default outputs does not run `tsc`.
  The real gate is the rules_ts-generated `//frontend:typecheck_typecheck_test`.
- A `vitest` target with no test files passes by finding nothing to run.

**A new gate is not trusted until it has been made to fail on purpose.** Break the
thing it claims to check, watch it go red, put it back. This is cheap and it is the
only evidence that distinguishes a gate from a decoration.

### 5. Adopt rulesets with the whole dependency tree in view

Before adding a ruleset, look at what it brings. `aspect_rules_js` brought a
telemetry collector; that is now opted out at the repo level in `.bazelrc`
(`common --repo_env=ASPECT_TOOLS_TELEMETRY_OPTOUT=1`) so the choice is a property
of the checkout rather than of whoever runs the build.

### 6. One generator per artifact

Where two tools can produce the same output, exactly one is authoritative and the
other is explicitly disabled with the reason written at the disable site. buf owns
protobuf codegen; `# gazelle:proto disable_global` turns off the competing path in
the root `BUILD.bazel`.

---

## Consequences

**Accepted costs:**
- Toolchain upgrades are lumpy. There is no cheap single-ruleset bump; budget for
  the batch.
- Two build paths exist per language — `go`/`pnpm` for the fast local loop, Bazel
  for the checkpoint. They can drift, so `make test` and `make bazel-test` must
  both run in CI. Where they disagree, Bazel is authoritative.
- Pinned SDKs mean a contributor's local Go or Node version is irrelevant to Bazel
  but still relevant to `make test`. That divergence is the point, and it is also a
  source of "works for me".

**Expected benefits:**
- The coupling table above is written down, so the next upgrade starts from
  knowledge rather than from the same five-step rediscovery.
- Version drift between the two build paths is structurally impossible for
  dependencies, because Bazel reads the same lockfiles rather than restating them.
- `bazel test //...` is a single hermetic signal across Go and TypeScript that does
  not depend on what is installed on the machine running it — which is what
  ADR-0001 asked for and did not have until now.

**Monitoring:** this ADR's coupling table is the living artifact. When a version
bump forces another, add the row. When a gate is found to be vacuous, add it to
§4's examples. The value is entirely in it being current.
