# T-030 offline observer evidence — 2026-09-14

Purpose: acceptance evidence for T-030 (demonstrate observer startup without
internet). Conclusions, procedure and limitations live in the durable
[T-030 runbook](../../../runbooks/validation/t030.md) and the task card; this
folder is the optional raw capture backing it and may be pruned per the
[artifact policy](../../README.md).

- Task: T-030, docs/tasks/cards/T-030-offline-observer.md
- Code revision: 137a100b392b1016584ad32fb43b76c033a6e7a9, plus the
  uncommitted MapPanel/FleetMap imagery-unavailable change this task added.
- Environment: Docker Compose (`gcs-backend` + stationary Copter 4.7.0 SITL,
  `GCS_RECORDING_ENABLED=true`), frontend dev server on the host
  (`pnpm dev --port 3001`), headless Chrome 153 driven over CDP
  (`--use-gl=angle --use-angle=swiftshader --enable-unsafe-swiftshader`).
- "Offline" method: `Network.setBlockedURLs` on the app's only external hosts
  (the three basemap tile providers), fresh `--user-data-dir` and
  `--disk-cache-dir=/dev/null` — real host networking was never touched.

Live SITL observations throughout; no synthetic fixtures or unexecuted checks.

## Files

Each numbered step has three files: `<n>-<name>.png` (full-page screenshot),
`<n>-<name>.innerText.txt` (text dump of `#root`, more diffable than the
image), `<n>-<name>.console.txt` (captured console/exception output).

| Step | What it shows | Expected | Observed |
|---|---|---|---|
| 01-online-baseline | Fleet overview, normal network | Esri tiles render | Matched; no unavailable chip |
| 02-online-vehicle | Vehicle workspace, normal network | Esri tiles render | Matched |
| 03-offline-fleet | Fleet overview, fresh profile, tiles blocked | Loads fully, explicit imagery gap, live telemetry | Matched — `Imagery unavailable` chip, live position/battery/link |
| 04-offline-vehicle | Vehicle workspace, tiles blocked | All instrument groups usable | Matched — link/state/position/guidance/heading/attitude/flight-data/raw all populated |
| 05-offline-mission | Mission download, tiles blocked | Mission downloads and overlays | Matched — waypoint table populated, `Commanded mission` chip alongside `Imagery unavailable` |
| 07/08-offline-replay | Replay of the recorded run (id 3, 4393 events), tiles blocked | Playback advances, telemetry updates, no imagery required | Matched — clock advanced 0:00→0:05, GPS/attitude/track updated |

No console exceptions in any step; only benign Vite/React-devtools/MapLibre
style-diff noise (the last is expected on a rapid style rebuild, not specific
to this change).
