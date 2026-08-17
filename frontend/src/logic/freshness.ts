/**
 * Age and staleness of a telemetry value.
 *
 * **Boundary convention: fresh when `nowMs - lastSeenMs < ttlMs`, strictly
 * less.** A value exactly at its TTL is stale. It is defined once here so
 * `FreshnessRing` and every other consumer share one boundary rather than
 * each inventing its own — two components disagreeing by one millisecond on
 * whether a vehicle is live is a real operator-facing bug.
 */

/** Milliseconds since a value was last seen. Never negative, never NaN. */
export function ageMs(lastSeenMs: number, nowMs: number): number {
  if (!Number.isFinite(lastSeenMs) || !Number.isFinite(nowMs)) {
    return Number.POSITIVE_INFINITY;
  }

  // A clock that steps backwards yields age 0, not a negative age. Callers
  // render this as a duration and a negative one is meaningless.
  return Math.max(0, nowMs - lastSeenMs);
}

/** True when the value is younger than its TTL. Exactly at TTL is stale. */
export function isFresh(lastSeenMs: number, nowMs: number, ttlMs: number): boolean {
  if (!Number.isFinite(ttlMs) || ttlMs <= 0) {
    return false;
  }

  return ageMs(lastSeenMs, nowMs) < ttlMs;
}
