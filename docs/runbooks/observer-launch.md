# Launch the local hardware observer

One command starts the backend and serves the built UI on one local port. No
SITL, no UDP input, no operator commands, no dev server. Hardware is attached
and chosen in the UI; nothing is configured by editing files.

**Verified on:** macOS 26.6.2 arm64 from a source checkout on a developer machine
(Go 1.25.0, Node 24.3.0, pnpm 10.30.1). Not verified on a clean machine, offline,
Linux or Windows; do not read this as support for them. A packaged install is
not part of this procedure.

## Prerequisites (first build only)

- Go 1.25+ with CGO: on macOS, `xcode-select --install`.
- Node and pnpm (`corepack enable`), to build the UI once.
- A USB data cable (not charge-only). macOS needs no driver for a native-USB
  ArduPilot controller; it appears as `/dev/cu.usbmodem*`.

After the first build the launcher starts without Go or Node as long as the
binary and `frontend/dist` exist. Installing dependencies needs the internet;
running the observer does not.

## Launch

```sh
./scripts/observer              # build if needed, start, open http://127.0.0.1:8080/
./scripts/observer --rebuild    # rebuild backend and UI first (after pulling)
./scripts/observer --no-open    # do not open a browser
```

Ctrl-C stops the backend, which closes any open serial port so another tool
(MissionPlanner) can take it.

## Defaults

| Setting | Value | Override |
|---|---|---|
| Address | `127.0.0.1:8080` (this machine only) | `GCS_HTTP_ADDR` |
| MAVLink UDP/SITL | off | `GCS_MAVLINK_UDP_BIND` |
| Operator commands | off | `GCS_COMMANDS_ENABLED=true` |
| Settings dir | `~/Library/Application Support/yalb-gcs/` | `GCS_DATA_DIR` |
| Saved connections | `<dir>/connection-profiles.json` | `GCS_CONNECTION_PROFILES_PATH` |
| Recordings | `<dir>/recordings.db` | `GCS_RECORDING_DB_PATH` |
| Backend log | `<dir>/observer.log` | — |

On macOS the device list also shows Bluetooth and debug ports; choose the
`usbmodem` entry. The first launch asks for nothing else.

## Startup failures

| Symptom | Meaning |
|---|---|
| `missing prerequisite 'go'` / `'pnpm'` | Install it (first build only). |
| `backend build failed` / `UI build failed` | Output tail is printed; UI log is `<dir>/ui-build.log`. |
| `port 8080 is already in use` + the owning process | Another observer or app holds it. Stop it, or use `GCS_HTTP_ADDR=127.0.0.1:<port>`. |
| `backend exited during startup` + log tail | Read `observer.log`. |
| Backend refuses `GCS_UI_DIR` | `frontend/dist` has no `index.html`; run with `--rebuild`. |

## Evidence so far (2026-10-06, developer machine, temp data dir, port 18080)

Built, started, served `/` and a client route with SPA fallback, `/api/*`
answered by the backend (unknown `/api` is a real 404), a second launch on the
same port was refused with the owner named, SIGTERM and foreground SIGINT both
stopped the backend (exit 130 for Ctrl-C). Not yet done: real USB controller,
empty-cache offline start, clean machine.
