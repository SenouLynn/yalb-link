# Development setup

## Prerequisites

- Go 1.25.x
- Bazelisk (reads `.bazelversion`, currently Bazel 8.7.0)
- Node.js 22 and Corepack/pnpm
- buf 1.72.x
- Docker with Compose v2
- golangci-lint v2

Verify the checkout with the versions pinned in the executable files rather
than copying versions from this guide:

```sh
go version
bazel version
pnpm --version
buf --version
docker compose version
```

Install frontend dependencies once:

```sh
cd frontend
pnpm install --frozen-lockfile
```

## Test and build

From the repository root:

```sh
make test
make lint-go
make bazel-test
```

Useful narrower checks:

```sh
make check-codec
make check-matrix
make check-integration SKIP_SITL_BUILD=1
```

## Generate code

```sh
make proto-gen
make bazel-tidy
```

Generated Go and TypeScript files are committed. Do not edit them directly.

## Run SITL

```sh
docker compose up
docker compose --profile multi-sitl up
docker compose --profile ui up
```

The default stack starts Copter SITL and the backend. SITL sends MAVLink
to `gcs-backend:14550`; the backend publishes UDP 14550 for host-side tools.
The UI profile currently starts the Vite scaffold, not a working flight UI.
