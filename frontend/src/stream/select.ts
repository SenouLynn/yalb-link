/** Chooses the stream implementation from the page's own URL. */

import type { TelemetryStream } from './events';
import { LiveEventSource } from './live';
import { MockEventSource } from './mock';

/** `?source=mock` is the only way to leave the live stream. */
export const MOCK_QUERY_VALUE = 'mock';

/**
 * Returns the stream this page should use.
 *
 * Live is the default and mock is opt-in, deliberately in that direction. A
 * ground station that quietly fell back to synthetic data when the backend was
 * unreachable would show a flying aircraft that does not exist, which is the
 * worst failure this display can have.
 */
export function selectStream(search: string): TelemetryStream {
  const source = new URLSearchParams(search).get('source');

  return source === MOCK_QUERY_VALUE ? new MockEventSource() : new LiveEventSource();
}

/** Reports whether this page is running on fixtures, for the UI to say so. */
export function isMockSource(search: string): boolean {
  return new URLSearchParams(search).get('source') === MOCK_QUERY_VALUE;
}
