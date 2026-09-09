# T-017 workspace acceptance — 2026-09-09

The user supplied the reviewed `alrighty-let-s-take-a-cozy-thompson.md` plan,
which carries operator confirmation of the reference identity. Independently
checked sibling `flight-path-hud` HEAD = `29426a9`. Browser comparison used its
`apps/gcs` vehicle view and its own mock bridge on isolated ports 18080/14560.
The reference screenshot's video is unavailable; video and Guided controls are
not capabilities supplied by YALB's backend. YALB keeps its graphite/amber
palette and primary readings; reference density, independent regions and thin
separators inform the composition. This is shell acceptance, not HUD parity.

## Executed setup

```sh
python3 -m venv /private/tmp/yalb-motion-venv
/private/tmp/yalb-motion-venv/bin/pip install pymavlink==2.4.49
# Recording's default Compose path failed with SQLite error 14 (T-021).
cat > /private/tmp/yalb-t017-compose.yml <<'YAML'
services:
  gcs-backend:
    environment:
      GCS_RECORDING_ENABLED: 'true'
      GCS_RECORDING_DB_PATH: /tmp/t017-recordings.db
YAML
docker compose -f docker-compose.yml -f /private/tmp/yalb-t017-compose.yml \
  --profile multi-sitl up -d --build gcs-backend ardupilot-sitl-copter-1 ardupilot-sitl-plane-2
/private/tmp/yalb-motion-venv/bin/python docs/runbooks/evidence/t017/preload-mission.py 5760
/private/tmp/yalb-motion-venv/bin/python docs/runbooks/evidence/t017/preload-mission.py 5761
# Use current host source, not a previously baked frontend image:
cd frontend && pnpm dev --port 3001
```

Port 3001 already had this checkout's Vite server, so it was reused after checking
the served source. Chrome used an isolated profile, `--headless=new`,
`--use-gl=angle --use-angle=swiftshader --enable-unsafe-swiftshader`, and
`--remote-debugging-port=9222`. CDP Emulation set exact 1440×900 and 768×1024
viewports. Readiness used wall-clock polling; no SSE network-idle wait.

## Expectations and results

Before judging: no document horizontal overflow in any of eight visibility
combinations; desktop map and all primary readings on screen together; mission
scroll internal. Narrow main scroll must reach every pane. Hide/show must keep
canvas identity and pending/complete mission state. Stale values stop driving
instruments at the existing 5 s TTL; mission timeout uses the existing 5 s
response deadline. After fixing terminal SSE reconnect, require recovery within
10 s of restarting a healthy backend. Local backend/browser clocks share the
host clock; JSON observations are timestamped UTC. No airborne accuracy claim.

- `layout.json`: all 16 viewport/visibility combinations have document width
  equal to viewport width, no runtime exceptions. Desktop instruments body
  fits (491 px content/client after rounding); mission content is taller than
  its body and scrolls internally. Narrow panes stack in scrollable main.
  Browser showed the original fixed-size heading clipped; bounded SVG sizing
  corrected it before these final captures.
- `live.json`: disarmed Copter 4.7.0 and Plane 4.6.3 at Compose home, speedup 1,
  normal binary MAVLink → current backend → SSE → browser. Each download
  completed with six ordered items, map route and active seq 0. Upload uses six
  slots: autopilot overwrites slot 0 with actual home (MSL ~10.1 m), followed
  by five waypoints at 21–25 m above home. Frame normalization from INT to
  equivalent non-INT global frame is expected. This is not a flown mission.
- Focused mission button → hide mission returned focus to its panel toggle.
  The immediate CDP `hidden` read in the console occurred before React commit;
  committed hidden state is covered by layout and lifecycle assertions.
  Same canvas survived map hide/show. Pending request survived visibility
  changes; switching to Plane cleared Copter's state and Plane downloaded.
- Harness `docker pause yalb-sitl-copter-1` stopped all its traffic, not a
  simulator flight input. At the 5.5 s observation all seven provenance groups
  were stale. A subsequent download visibly timed out; unpause plus a new
  request completed without backend restart. No synthetic telemetry used.
- Backend stop caused DISCONNECTED and stale readings. Original EventSource
  remained permanently closed after proxy HTTP 500 despite healthy backend.
  `recovery-replay.json` records the fixed rerun: recovery in 1010 ms, under
  the 10 s bound. Closed sources now retry every 2 s; native reconnect remains
  in charge of CONNECTING sources. Unit regression covers retry cancellation.
  The earlier `backend-reconnected` entry in `live.json` is the **failed
  pre-fix observation**, not a passing result.
- Recording 1, `T-017 workspace live acceptance`, 3366 events, stopped cleanly
  on backend shutdown and replayed after restart. Play advanced the clock,
  pause stopped it, toggles preserved transport, one replay bar stayed in
  topbar at both sizes without overflow. Initial replay frames had absent
  heartbeat/battery; unavailable posture remained visible until recorded
  data arrived. Mission snapshots are not recorded: replay begins “Not
  downloaded”; its existing download action queries live backend state.
- Guarded command confirmation/busy/unresolved ownership and A → B isolation
  are tested with controlled HTTP responses, not actual arming. Sidebar
  commands are keyed by identity to prevent a confirmation carrying to B.

Raw working artifacts, CDP drivers and the exported SQLite database remain at
`/private/tmp/yalb-t017-evidence/` (ephemeral); checked-in JSON and screenshots
are the durable acceptance summary. SQLite was exported after clean recording
shutdown. This does not prove persistence across container recreation: T-021
owns named-volume setup.

## Verification

Unchanged code: frontend typecheck/lint, 281 tests, build all exit 0;
`go test -race ./...` exit 0, including recording. The first sandboxed Go attempt
could not access its cache; the permitted rerun passed. Final frontend results
are in the development runbook. Build retains the pre-existing large-bundle
warning. Bazel 8.3.1 refuses repo pin 8.7.0; golangci-lint is absent, so those
gates are unexecuted. Do not count them as passes.

At the T-017 completion boundary T-008 still needed exact-value/empty-mission
acceptance; the separate follow-through below completed it.
T-012 owns route framing, T-014 mission vocabulary, T-015 inspection below the
instruments primary tier, T-016 a future messages pane in main, T-013 HUD and
raw-stream parity. T-020 tracks unchanged-mission marker churn.

## T-008 follow-through

After closing T-017, claimed T-008 separately. Ran `compare-missions.py` with
pymavlink 2.4.49 against ports 5760 and 5761. It independently requested each
MISSION_ITEM_INT from each autopilot, then compared the backend HTTP download:
six ordered items, exact seq/command/autocontinue/params, lat/lon within 1e-7°
and altitude within 1 mm of decoded wire float. `mission-comparison.json`
contains both wire messages and backend values, including normalized frames:
0 = GLOBAL for home, 3 = GLOBAL_RELATIVE_ALT for subsequent items. The
externally loaded waypoint coordinates are 37.7749 + seq×0.0002,
-122.4194 + (seq mod 2)×0.0003, altitude 20 + seq m (seq 1–5). Both returned
those values; home seq 0 is autopilot-owned and is compared with the received
wire value, not the setup placeholder. The live browser showed ordered labels
0–5, connected route and active seq 0 for both vehicles; source geometry ordering
also retains the existing frontend regression coverage.

Then ran `preload-mission.py 5760 0` and `preload-mission.py 5761 0` externally.
Copter's first clear timed out and retained six items; a separate retry received
an accepted ACK. Plane cleared on the first attempt. No application retry
policy changed. Final `zero-missions.json` records both live UI downloads as
“Complete · empty”, no commanded-route key and zero mission markers. This also
verifies empty differs from home-only; ArduPilot can report true zero here.
The controlled application timeout and successful subsequent request are in
`live.json`. No simulator was armed or flown for these inspection checks.
T-008's acceptance is complete for the ordinary mission case described here.

Browser version: Chrome 153.0.8010.36. `reachability.json` records narrow main
scroll 602 px and mission scroll 122 px, with the final item bottom at 1001.89 px
inside the 1024 px viewport. `narrow-scrolled.png` captures that reachable end.

Final T-008 verification: `make test` exit 0 (all Go packages with race
detector, 32 frontend files / 287 tests). `./scripts/kanban check` and
`git diff --check` exit 0. No commits or deployment were requested.
