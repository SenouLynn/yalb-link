/**
 * MapLibre paints from resolved values, not custom properties, so the mission
 * colour exists here too. `--mission-route` in `display.css` is the definition
 * a document actually renders from; this is the fallback used when a map is
 * mounted somewhere that stylesheet has not reached, and must match it.
 */
export const MISSION_ROUTE_COLOR = '#c7f0ff';
