# UI reference review

Source review on 2026-09-08: local sibling `flight-path-hud`, primarily
`apps/gcs/src`. Its origin is `SenouLynn/flight-path-hud`. The user-supplied reviewed T-017 plan (2026-09-09) carries operator
confirmation that this is the named `flight-hud-trajectory` reference, pinned
to `29426a9`, primarily `apps/gcs/src`. These are
code observations, not browser-verified visual acceptance.

| Surface | Reference evidence | YALB state / implication |
|---|---|---|
| Fleet → vehicle workspace | `App.tsx`, `fleet/FleetView.tsx`, `NodeView.tsx` | Fleet state and selection exist; no fleet overview map/roster workspace. |
| Persistent shell and feed | `App.tsx` owns feed above view switch | Preserve YALB's stream and fleet ownership in `frontend/src/App.tsx`. |
| Compact context sidebar | `NodeView.tsx`, `index.css` | Current status, selection and arm controls are stacked above instruments. Group by operator purpose. |
| Selectable panes | `ViewsMenu.tsx`, named grid areas in `index.css` | Current display is a 60rem vertical stack. Establish responsive workspace composition before adding readouts. |
| Integrated instruments | `hud/HudPanel.tsx` | Reference pairs heading with unified HUD, orientation and Cartesian track; YALB has attitude, heading, numeric readouts and geographic prediction. Instrument parity needs its own comparison. |
| Telemetry inspection | `log/ParametersPanel.tsx` | Fixed, filtered telemetry-field table, despite the Parameters label; not an autopilot parameter editor. T-015 identifies already decoded but unrendered fields. |
| Message inspection | `log/LogsPanel.tsx`, `log/LogPanel.tsx` | Raw stream tab has timestamps, vehicle identity, message name, sequence, filtering, pause and clear. T-016's STATUSTEXT log alone does not provide this. |
| Mission/map interaction | `NodeView.tsx` | Selecting a waypoint pans the map and releases follow. T-012 covers initial mission framing; YALB already has download/list/route state. |
| Video and guided workflows | `video/VideoPanel.tsx`, `NodeView.tsx` | Additional capabilities, not buttons to copy into an unsupported UI. Current YALB command surface is guarded arm/disarm only. |

Trajectory-era commits `bccf549`, `9288986`, and `5f73e30` survive in
`packages/hud-ui/src/components/HudPredictiveTrajectory.tsx`; the current GCS
uses `HudUnifiedInstrument` instead. T-013 owns instrument parity. Guided and
video panels assume backend capabilities YALB does not expose; their exclusion
is a backend constraint, not merely a layout choice.

## Recovered motion profiles

Source: sibling `flight-path-hud` at commit `29426a9`,
`apps/mavlink-bridge/src/flightProfiles.js`, `mockNodes.js`,
`mockFleetRunner.js` and `flightProfiles.test.js`.

- Snake: integrated travel at a nominal 18 m/s, heading oscillating ±20° about
  180° with angular frequency 0.25 rad/s. Bank is derived from turn rate;
  pitch, speed and climb also vary. The route continues away from its origin.
- Figure-eight: analytic position on a Gerono lemniscate, spanning 300 m east/
  west and 200 m north/south, repeating every 60 s without integration drift.
  Height varies around 100 m by ±25 m. Velocity, turn rate and bank derive
  from the curve; pitch is approximated by flight-path angle. These are
  synthetic kinematics, not an aerodynamic aircraft model.
- The roster assigns snake system 1 and figure-eight system 2 separate origins
  and missions. The figure-eight mission samples four lobe extremes.
- The runner serializes envelopes as JSON datagrams over UDP. It does not run
  ArduPilot SITL or emit the binary MAVLink frames YALB's receiver expects.

These patterns are useful inputs for repeatable validation design. T-018 plans
SITL adaptation; no equivalent SITL scenarios have been demonstrated here.

## Existing constraints to preserve

ADRs 0001–0003 establish SQLite recording, bounded resource use, deterministic
replay and retention. ADR 0004 defines addressed arm transactions, ambiguous
outcomes, operator attestation and quarantine. A UI redesign does not require
replacing those choices.

`frontend/src/ui/readings.ts` and `FlightDisplay.tsx` enforce absent/stale
posture and gate prediction. Preserve those semantics, source identification,
full vehicle identity and mission request lifecycle when composing new panels.

Board evidence: T-001–T-003 record trajectory acceptance; T-004–T-011 contain
completed mission implementation and hardening except T-008, whose live
Copter/Plane mission acceptance remains ready and incomplete. This review did
not rerun their verification and does not claim new live acceptance.
