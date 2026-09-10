import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, it } from 'vitest';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { HeartbeatStateSchema } from '@/gen/gcs/v1/vehicle_pb';
import { initialFleetState, fleetReducer } from '@/fleet/state';
import { mockFrames } from '@/stream/fixtures';
import { StateRows } from './StatusBar';

it('ages heartbeat state from its observation without expiring unchanged state or resetting on LOST', () => {
  const now = 1_700_000_000_000;
  const frame = mockFrames(1)[0];
  if (!frame) throw new Error('Missing bootstrap fixture');
  const state = fleetReducer(initialFleetState, { type: 'stream', event: frame.build(now) });
  const view = state.vehicles['1:1'];
  if (!view) throw new Error('Missing vehicle');
  const snapshot = { ...view, heartbeat: create(HeartbeatStateSchema, { armed: true, observedAt: timestampFromMs(now - 30_000) }) };
  const props = { view: snapshot, nowMs: now, connected: true, source: 'live' as const };
  const unchanged = renderToStaticMarkup(<StateRows {...props} />);
  expect(unchanged).toContain('HEARTBEAT STATE');
  expect(unchanged).toContain('30s');
  expect(unchanged).not.toContain('provenance--stale');
  const lost = renderToStaticMarkup(<StateRows {...props} view={{ ...snapshot, lifecycle: FleetEventType.VEHICLE_LOST, lastFleetAtMs: now }} />);
  expect(lost).toContain('30s');
  expect(lost).toContain('provenance--stale');
});
