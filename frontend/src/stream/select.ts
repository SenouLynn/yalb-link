/** Chooses the stream implementation from the page's own URL. */

import type { TelemetryStream } from './events';
import { LiveEventSource } from './live';
import { MockEventSource } from './mock';
import { ReplayEventSource } from './replay';

/** Where the display's data is coming from. Named, never inferred. */
export type StreamSource = 'live' | 'mock' | 'replay';

/** `?source=mock` replays fixtures; `?source=replay` replays a recording. */
export const MOCK_QUERY_VALUE = 'mock';
export const REPLAY_QUERY_VALUE = 'replay';

/** Which recording to replay. */
export const RECORDING_QUERY_KEY = 'recording';

/** The chosen stream, and what the display must say about it. */
export interface StreamSelection {
  stream: TelemetryStream;
  source: StreamSource;
  /**
   * The replay source when one was selected, for the transport controls.
   * Null in replay mode too, when no recording has been chosen yet.
   */
  replay: ReplayEventSource | null;
}

/** A stream that delivers nothing, for replay with no recording chosen. */
class IdleStream implements TelemetryStream {
  start(): () => void {
    return () => {
      // Nothing was started, so there is nothing to stop.
    };
  }
}

/**
 * Returns the stream this page should use.
 *
 * Live is the default and everything else is opt-in, deliberately in that
 * direction. A ground station that quietly fell back to synthetic or recorded
 * data when the backend was unreachable would show a flying aircraft that does
 * not exist, which is the worst failure this display can have. The same rule
 * runs the other way: a page asked for a replay it cannot load stays in replay
 * mode and says so, rather than silently showing live telemetry instead.
 */
export function selectStream(search: string): StreamSelection {
  const params = new URLSearchParams(search);
  const source = params.get('source');

  if (source === MOCK_QUERY_VALUE) {
    return { stream: new MockEventSource(), source: 'mock', replay: null };
  }

  if (source === REPLAY_QUERY_VALUE) {
    const recordingId = parseRecordingId(params.get(RECORDING_QUERY_KEY));

    if (recordingId === null) {
      return { stream: new IdleStream(), source: 'replay', replay: null };
    }

    const replay = new ReplayEventSource({ recordingId });

    return { stream: replay, source: 'replay', replay };
  }

  return { stream: new LiveEventSource(), source: 'live', replay: null };
}

/** Reads the recording id. Anything that is not a positive integer is absent. */
export function parseRecordingId(raw: string | null): number | null {
  if (raw === null || !/^\d+$/.test(raw)) {
    return null;
  }

  const value = Number(raw);

  return Number.isSafeInteger(value) && value > 0 ? value : null;
}

/** Builds the URL that replays one recording. */
export function replayUrl(recordingId: number): string {
  return `?source=${REPLAY_QUERY_VALUE}&${RECORDING_QUERY_KEY}=${String(recordingId)}`;
}
