/** The live backend stream, over the browser's own SSE client. */

import { EVENT_FLEET, EVENT_TELEMETRY, type StreamEvent, type TelemetryStream } from './events';
import { parseStreamEvent } from './parse';

/**
 * Where the backend stream is mounted.
 *
 * Relative on purpose. Vite proxies `/api` in development and the same origin
 * serves it in production, so the client needs no configured backend URL and
 * the backend needs no CORS policy.
 */
export const EVENTS_PATH = '/api/events';

export interface LiveOptions {
  /** Overridable for tests. Defaults to the page's own origin. */
  url?: string;
  /** Injected clock. Receipt time is the browser's, not the vehicle's. */
  now?: () => number;
  /** Injected for tests; defaults to the DOM's EventSource. */
  create?: (url: string) => EventSource;
}

/**
 * LiveEventSource follows `/api/events`.
 *
 * Reconnection is left to the browser, which retries an EventSource on its
 * own. The backend's contract makes that safe: every reconnect gets a fresh
 * bootstrap of retained state, so a resumed stream cannot leave the display
 * showing values from before the gap.
 */
export class LiveEventSource implements TelemetryStream {
  private readonly url: string;
  private readonly now: () => number;
  private readonly create: (url: string) => EventSource;

  constructor(options: LiveOptions = {}) {
    this.url = options.url ?? EVENTS_PATH;
    this.now = options.now ?? (() => Date.now());
    this.create = options.create ?? ((url) => new globalThis.EventSource(url));
  }

  start(onEvent: (event: StreamEvent) => void): () => void {
    const source = this.create(this.url);

    const forward = (name: string) => (message: MessageEvent<string>) => {
      const parsed = parseStreamEvent(name, message.data, this.now());

      if (parsed !== null) {
        onEvent(parsed);
      }
    };

    const onFleet = forward(EVENT_FLEET);
    const onTelemetry = forward(EVENT_TELEMETRY);

    const onOpen = () => {
      onEvent({ kind: 'connection', connected: true, receivedAtMs: this.now() });
    };

    // EventSource reports every failure as a bare `error`, including the ones
    // it is about to retry. Reporting it as a disconnect is correct either
    // way: until the next `open`, the browser is not receiving telemetry.
    const onError = () => {
      onEvent({ kind: 'connection', connected: false, receivedAtMs: this.now() });
    };

    source.addEventListener(EVENT_FLEET, onFleet as EventListener);
    source.addEventListener(EVENT_TELEMETRY, onTelemetry as EventListener);
    source.addEventListener('open', onOpen);
    source.addEventListener('error', onError);

    return () => {
      source.removeEventListener(EVENT_FLEET, onFleet as EventListener);
      source.removeEventListener(EVENT_TELEMETRY, onTelemetry as EventListener);
      source.removeEventListener('open', onOpen);
      source.removeEventListener('error', onError);
      source.close();
    };
  }
}
