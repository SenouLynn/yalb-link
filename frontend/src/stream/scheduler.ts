/** Deferred execution, injectable so tests need no timers. */

/** Cancels a scheduled callback. */
export type Cancel = () => void;

/** Schedules a callback. */
export type Scheduler = (fn: () => void, delayMs: number) => Cancel;

/** The browser's own timer. */
export const defaultScheduler: Scheduler = (fn, delayMs) => {
  const handle = globalThis.setTimeout(fn, delayMs);

  return () => {
    globalThis.clearTimeout(handle);
  };
};
