import { useEffect, useReducer, useState } from 'react';

import { useConnections } from '@/connections/useConnections';
import { fleetReducer, initialFleetState, type VehicleKey } from '@/fleet/state';
import { selectStream } from '@/stream/select';
import { FlightDisplay } from '@/ui/FlightDisplay';

/**
 * How often the display re-evaluates freshness.
 *
 * Independent of the event rate on purpose: a value goes stale because nothing
 * arrived, so the thing that has to notice cannot be driven by arrivals. Four
 * times a second is fast enough that the drain reads as continuous and slow
 * enough to be invisible in a profile.
 */
const FRESHNESS_TICK_MS = 250;

export default function App() {
  const search = globalThis.location.search;

  const [fleet, dispatch] = useReducer(fleetReducer, initialFleetState);
  const [nowMs, setNowMs] = useState(() => Date.now());

  // The stream is built once per page: rebuilding it would drop the SSE
  // connection and force a fresh bootstrap on every render.
  const [{ stream, source, replay }] = useState(() => selectStream(search));

  // A second, independent fold off the same feed (ADR 0006: a connection is
  // an acquisition path, not a vehicle identity, and the two fail
  // independently) — see connections/state.ts.
  const connections = useConnections(source === 'live');

  // `dispatchStream` is stable (`useCallback` with no deps in `useConnections`);
  // depending on the whole `connections` object here would rebuild the stream
  // — and drop the SSE connection — on every render, since its returned object
  // is a fresh literal each time.
  const { dispatchStream } = connections;
  useEffect(() => stream.start((event) => {
    dispatch({ type: 'stream', event });
    dispatchStream(event);
  }), [stream, dispatchStream]);

  useEffect(() => {
    const handle = globalThis.setInterval(() => {
      // A source that owns a clock supplies it. Replay does, because its
      // telemetry is stamped with the time of the flight that produced it and
      // would read as hours stale against the wall clock.
      setNowMs(stream.now?.() ?? Date.now());
    }, FRESHNESS_TICK_MS);

    return () => {
      globalThis.clearInterval(handle);
    };
  }, [stream]);

  const select = (key: VehicleKey) => {
    dispatch({ type: 'select', key });
  };

  return (
    <FlightDisplay
      fleet={fleet}
      nowMs={nowMs}
      source={source}
      replay={replay}
      onSelect={select}
      connections={connections}
    />
  );
}
