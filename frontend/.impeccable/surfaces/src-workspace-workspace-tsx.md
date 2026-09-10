---
version: 1
slug: "src-workspace-workspace-tsx"
primary_target: "src/workspace/Workspace.tsx"
related_targets: ["src/ui/display.css","src/ui/FlightDisplay.tsx"]
---

## Scope

The vehicle workspace shell and the panel vocabulary every panel is built from:
`src/workspace/`, `src/ui/display.css`, `src/ui/FlightDisplay.tsx`, and the
panels those compose. Visitor mode: **Operate**.

Audience: solo builder-operator at a desk (field later), plus the developer as a
second audience on the same screen with a higher information ceiling.

Task: read a fleet, focus one node, and read that node's entire state at a
glance while keeping its command levers in reach.

Constraints: one viewport, no page scroll; three interchangeable sources (live
SSE, `?source=mock`, replay); state is stream-shaped passthrough and components
are clients of it.

## Direction contract

**THESIS:** The screen is a declaration of the telemetry stream, not a composed
page. Every value is a labeled row that names its source and its age; the layout
is an assembly of registered panels. It refuses the card-grid dashboard of
hero metrics and generous whitespace that the current build drifted into —
here, whitespace that carries no information is screen area stolen from data.

**OWN-WORLD:** DearImGui's chrome in flight-path-hud's instrument palette. Flat
graphite panels butted edge to edge on 1px bezel rules; radius never above 2px;
no shadows, no floating cards, no gaps between panels. The atom is a two-column
row: muted uppercase label left, monospace tabular value right-aligned against a
common edge, unit dimmer. Tiny letter-spaced section headers over hairline
rules. Rectangular 1px-bordered controls in horizontal lever rows. Amber is
caution only — never chrome, never focus decoration. No "good" green.

**STORY:** The operator sees what is on the link, focuses a node, and reads its
whole state — link, target state, mission, position, trail, instruments, and the
raw decoder tier — resident at once, without scrolling or clicking. Levers sit
beside the data they act on, so commanding never means going to look for a
control.

**FIRST VIEWPORT:** App bar (`GROUND CONTROL`, operator identity right) over a
view subbar (`← Fleet`, scope picker, `Views (n)`). Below, three columns butted
on 1px rules: a fixed ~240px left rail of stacked labeled groups, each a
two-column key→value table; a center column with a horizontal command lever row
above the map and a guided-command form below it; a right column of instruments
over a permanent `Parameters | Raw stream` tab strip. Primary action is the
lever row directly above the map. Nothing floats; nothing is centered in void.

**FORM:** Pinned by the brief, not selected from a candidate list — DearImGui as
philosophy, `~/Desktop/flight-path-hud/apps/gcs` as the working implementation
to draw from. No concept roll was run and there is no seed key; redirecting a
twice-pinned world would be failure, not diligence. Resolved with the owner:
panel registry over fixed named grid areas (no docking engine); developer tier
always visible and tabbed; data maximally dense while interactive hit targets
stay full-size — density and touch-safety are separate axes, expressed as
separate tokens.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the
finish review, the verdict, DESIGN.md, and every shipping raster carrying its
provenance

## Unresolved

- Whether operator panel/layout choices persist across reload.
- The final composition of the screen (unsettled by intent).
