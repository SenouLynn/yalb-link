/**
 * Test helpers shared across resolver suites.
 *
 * Not a fixture table — the tables live in `__fixtures__/` and hold the actual
 * test vectors. This only supplies the two envelope fields every sample
 * carries so the tables can stay focused on the fields under test.
 */

import type { TelemetrySample } from './sample';

/** Fixed clock. Tests never call Date.now(); resolvers stay deterministic. */
export const FIXED_NOW_MS = 1_700_000_000_000;

/** Completes a partial fixture input into a full TelemetrySample. */
export function baseSample(overrides: Partial<TelemetrySample> = {}): TelemetrySample {
  return {
    sourceMessage: 'TEST',
    receivedAtMs: FIXED_NOW_MS,
    ...overrides,
  };
}
