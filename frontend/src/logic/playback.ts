/**
 * The replay clock.
 *
 * Replay needs a clock the display can be measured against, because freshness
 * is judged from the backend's `observed_at` rather than from browser receipt
 * time. Recorded telemetry carries the timestamps of the flight that produced
 * it, so a replay driven by `Date.now()` would render every reading as stale
 * and show an entirely blank display. `positionMs` is therefore an absolute
 * epoch time *on the recording's own timeline*, not an offset into it: it is
 * fed straight to the same freshness rules the live display uses.
 *
 * Everything here is pure, for the same reason the resolvers are: playback
 * ordering and seeking are worth testing without a DOM, a timer, or a network.
 */

/** Anything the clock can schedule. */
export interface TimedItem {
  /** Absolute epoch milliseconds, on the recording's timeline. */
  atMs: number;
}

/** Where playback is, and how it is moving. */
export interface PlaybackState {
  /** Absolute epoch ms on the recording's timeline. */
  positionMs: number;
  startedAtMs: number;
  endedAtMs: number;
  /** Wall-time multiplier. Always finite and greater than zero. */
  speed: number;
  playing: boolean;
}

/** The initial clock for a recording: paused, at the beginning, real time. */
export function createPlayback(startedAtMs: number, endedAtMs: number): PlaybackState {
  return {
    positionMs: startedAtMs,
    startedAtMs,
    endedAtMs: Math.max(startedAtMs, endedAtMs),
    speed: 1,
    playing: false,
  };
}

/**
 * Moves the clock on by a wall-time delta.
 *
 * Returns the same object when nothing moves, so a paused clock ticking at
 * 4 Hz does not look to React like a changed one.
 */
export function advance(state: PlaybackState, wallDeltaMs: number): PlaybackState {
  if (!state.playing || !Number.isFinite(wallDeltaMs) || wallDeltaMs <= 0) {
    return state;
  }

  const target = state.positionMs + wallDeltaMs * state.speed;

  if (target >= state.endedAtMs) {
    return { ...state, positionMs: state.endedAtMs, playing: false };
  }

  return { ...state, positionMs: target };
}

/** Resumes playback, restarting from the beginning if it had finished. */
export function play(state: PlaybackState): PlaybackState {
  if (state.positionMs >= state.endedAtMs) {
    return { ...state, positionMs: state.startedAtMs, playing: true };
  }

  return { ...state, playing: true };
}

/** Holds playback where it is. */
export function pause(state: PlaybackState): PlaybackState {
  return { ...state, playing: false };
}

/** Moves to an absolute position, clamped into the recording. */
export function seekTo(state: PlaybackState, positionMs: number): PlaybackState {
  if (!Number.isFinite(positionMs)) {
    return state;
  }

  const clamped = Math.min(Math.max(positionMs, state.startedAtMs), state.endedAtMs);

  return { ...state, positionMs: clamped };
}

/** Sets the wall-time multiplier. A non-positive speed is not a speed. */
export function setSpeed(state: PlaybackState, speed: number): PlaybackState {
  if (!Number.isFinite(speed) || speed <= 0) {
    return state;
  }

  return { ...state, speed };
}

/**
 * Grows the end of the recording as later pages arrive.
 *
 * The full extent is not known until paging finishes, and the end must never
 * retreat behind where playback already is.
 */
export function extendTo(state: PlaybackState, endedAtMs: number): PlaybackState {
  if (!Number.isFinite(endedAtMs) || endedAtMs <= state.endedAtMs) {
    return state;
  }

  return { ...state, endedAtMs };
}

/**
 * How many items starting at `cursor` are due at or before `throughMs`.
 *
 * Counts a leading run and stops at the first item that is not yet due, even
 * if a later one would be. Recorded order is delivery order rather than
 * timestamp order, and emitting a later event early to catch up would reorder
 * telemetry that the live path delivered in sequence. Holding it for one tick
 * is the cheaper error.
 */
export function dueCount(items: TimedItem[], cursor: number, throughMs: number): number {
  let count = 0;

  for (let index = Math.max(0, cursor); index < items.length; index += 1) {
    const item = items[index];

    if (item === undefined || item.atMs > throughMs) {
      break;
    }

    count += 1;
  }

  return count;
}
