# T-021 Compose recording persistence

Executed 2026-09-09 with Docker Compose and stationary Copter 4.7.0 SITL.
The Compose project initially had no running services or recording volume.

1. `GCS_RECORDING_ENABLED=true docker compose up -d --build --wait gcs-backend`
   created `yalb-gcs_recordings` and reached healthy status. `docker compose exec
   -T gcs-backend id` reported UID/GID 10001; database, WAL and SHM files under
   `/var/lib/gcs` were owned by 10001.
2. POST `/api/recordings/start` with name `T-021 persistence acceptance`, then
   `GCS_RECORDING_ENABLED=true docker compose up -d --no-build --wait ardupilot-sitl-copter-1`.
   No arm or motion commands were sent.
3. POST `/api/recordings/stop` flushed 651 events. Save GET
   `/api/recordings/1/events?limit=1000` as `before.json`.
4. `GCS_RECORDING_ENABLED=true docker compose up -d --force-recreate --wait gcs-backend`
   reached healthy status. GET `/api/recordings` retained session 1 and its count.
   Save the same replay endpoint as `after.json`.
5. Compare parsed JSON: every event and all lifecycle metadata match;
   `next_seq` is null so the page contains the complete recording.
6. `GCS_RECORDING_ENABLED=false docker compose up -d --force-recreate --wait gcs-backend`
   reached healthy status; `/healthz` succeeded and `/api/recordings` returned 404.
7. `docker compose down` restored the initially stopped stack; the named volume
   and acceptance session remain. No recording volumes were deleted.

Artifacts: [stopped session](stopped.json), [before recreation](before.json),
[after recreation](after.json), [comparison](comparison.json).

Repeat the replay assertion from the repository root:

```sh
python3 - <<'PYTHON'
import json
from pathlib import Path
root = Path('docs/runbooks/evidence/t021')
before = json.loads((root / 'before.json').read_text())
after = json.loads((root / 'after.json').read_text())
assert before == after
assert len(before['events']) == 651
assert before['next_seq'] is None
PYTHON
```

This proves storage permission and full API replay persistence across container
recreation. It does not add a browser replay or flight-dynamics acceptance claim.
The documentation's destructive reset commands were not executed.
