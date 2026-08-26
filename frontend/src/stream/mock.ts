/** Deterministic playback of the fixture script, with no network involved. */

import type { StreamEvent, TelemetryStream } from './events';
import { mockFrames, type MockFrame } from './fixtures';
import { defaultScheduler, type Cancel, type Scheduler } from './scheduler';

export type { Cancel, Scheduler } from './scheduler';

export interface MockOptions {
  /** Injected clock. Defaults to the wall clock so freshness still ages. */
  now?: () => number;
  /** Injected scheduler. Defaults to setTimeout. */
  schedule?: Scheduler;
  /** Fixture script. Defaults to the standard flight. */
  frames?: MockFrame[];
  /** Repeat the script rather than stopping at the end. */
  loop?: boolean;
}

/**
 * MockEventSource replays fixtures through the same interface as the live
 * stream.
 *
 * It exists so the display can be demonstrated and tested without a backend,
 * and so a failing instrument can be reproduced from a fixed input rather than
 * from whatever a vehicle happened to be doing.
 */
export class MockEventSource implements TelemetryStream {
  private readonly wallNow: () => number;
  private readonly schedule: Scheduler;
  private readonly frames: MockFrame[];
  private readonly loop: boolean;

  constructor(options: MockOptions = {}) {
    this.wallNow = options.now ?? (() => Date.now());
    this.schedule = options.schedule ?? defaultScheduler;
    this.frames = options.frames ?? mockFrames();
    this.loop = options.loop ?? true;
  }

  start(onEvent: (event: StreamEvent) => void): Cancel {
    let stopped = false;
    let cancelPending: Cancel | null = null;
    let index = 0;

    // Mock playback is always "connected": there is no transport to lose, and
    // claiming otherwise would make the demo show a connection warning that
    // does not correspond to anything.
    onEvent({ kind: 'connection', connected: true, receivedAtMs: this.wallNow() });

    const step = (): void => {
      if (stopped || this.frames.length === 0) {
        return;
      }

      const frame = this.frames[index % this.frames.length];

      if (frame === undefined) {
        return;
      }

      onEvent(frame.build(this.wallNow()));

      index += 1;

      if (!this.loop && index >= this.frames.length) {
        return;
      }

      cancelPending = this.schedule(step, frame.delayMs);
    };

    step();

    return () => {
      stopped = true;
      cancelPending?.();
    };
  }
}
