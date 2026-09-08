# UI reference review

Source review on 2026-09-08: local sibling `flight-path-hud`, primarily
`apps/gcs/src`. Its origin is `SenouLynn/flight-path-hud`. Confirmation that this
is the operator's named `flight-hud-trajectory` reference is pending. These are
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
