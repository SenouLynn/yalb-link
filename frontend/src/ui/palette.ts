/**
 * MapLibre paints from resolved values, not custom properties, so the mission
 * colour exists here too. `--mission-route` in `system.css` is the definition
 * a document actually renders from; this is the fallback used when a map is
 * mounted somewhere that stylesheet has not reached, and must match it.
 */
export const MISSION_ROUTE_COLOR = '#c7f0ff';

/**
 * The projected path.
 *
 * Deliberately not the caution amber it used to be: amber on this display means
 * something is wrong with the aircraft, and a five-second prediction is not a
 * hazard. Mirrors `--prediction` in `system.css`.
 */
export const PREDICTION_COLOR = '#9fb4c9';

/** Resolved mirrors for MapLibre paint. Pinned against system.css by tests. */
export const TRACK_COLOR = '#b6a6d9';
export const PANEL_COLOR = '#14171c';
export const PANEL_DEEP_COLOR = '#0d1013';
