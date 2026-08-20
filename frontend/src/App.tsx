import { useEffect, useReducer, useState } from 'react';

import { fleetReducer, initialFleetState, type VehicleKey } from '@/fleet/state';
import { isMockSource, selectStream } from '@/stream/select';
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
  const [stream] = useState(() => selectStream(search));
  const [mock] = useState(() => isMockSource(search));

  useEffect(() => stream.start((event) => {
    dispatch({ type: 'stream', event });
  }), [stream]);

  useEffect(() => {
    const handle = globalThis.setInterval(() => {
      setNowMs(Date.now());
    }, FRESHNESS_TICK_MS);

    return () => {
      globalThis.clearInterval(handle);
    };
  }, []);

  const select = (key: VehicleKey) => {
    dispatch({ type: 'select', key });
  };

  return <FlightDisplay fleet={fleet} nowMs={nowMs} mock={mock} onSelect={select} />;
}
