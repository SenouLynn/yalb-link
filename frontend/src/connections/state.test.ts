import { describe, expect, it } from 'vitest';

import { connectionList, connectionsReducer, initialConnectionsState } from './state';
import type { ConnectionStatus } from './types';

function status(partial: Partial<ConnectionStatus> & Pick<ConnectionStatus, 'id' | 'state'>): ConnectionStatus {
  return { vehicleKeys: [], ...partial };
}

describe('connectionsReducer', () => {
  it('bootstraps the whole fold from a GET /api/connections read', () => {
    const state = connectionsReducer(initialConnectionsState, {
      type: 'bootstrap',
      statuses: [status({ id: 'a', state: 'REPORTING' }), status({ id: 'b', state: 'IDLE' })],
    });

    expect(connectionList(state).map((s) => s.id)).toEqual(['a', 'b']);
  });

  it('replaces the fold outright on a second bootstrap — a stale entry not named is gone', () => {
    const first = connectionsReducer(initialConnectionsState, {
      type: 'bootstrap', statuses: [status({ id: 'a', state: 'IDLE' }), status({ id: 'b', state: 'IDLE' })],
    });
    const second = connectionsReducer(first, { type: 'bootstrap', statuses: [status({ id: 'a', state: 'REPORTING' })] });

    expect(connectionList(second).map((s) => s.id)).toEqual(['a']);
  });

  it('upserts an acquisition event, appending a new connection to stable order', () => {
    const bootstrapped = connectionsReducer(initialConnectionsState, {
      type: 'bootstrap', statuses: [status({ id: 'a', state: 'IDLE' })],
    });
    const withUpdate = connectionsReducer(bootstrapped, {
      type: 'stream',
      event: { kind: 'acquisition', event: status({ id: 'a', state: 'REPORTING' }), receivedAtMs: 1 },
    });
    const withNew = connectionsReducer(withUpdate, {
      type: 'stream',
      event: { kind: 'acquisition', event: status({ id: 'b', state: 'OPENING' }), receivedAtMs: 2 },
    });

    expect(connectionList(withNew)).toEqual([
      status({ id: 'a', state: 'REPORTING' }),
      status({ id: 'b', state: 'OPENING' }),
    ]);
  });

  it('a late subscriber sees current per-connection state, not just changes since it joined', () => {
    // Simulates the hub's bootstrap-on-connect behavior (contract §6): the
    // first events this browser ever sees for an id are still authoritative.
    const state = connectionsReducer(initialConnectionsState, {
      type: 'stream',
      event: { kind: 'acquisition', event: status({ id: 'a', state: 'REPORTING' }), receivedAtMs: 1 },
    });

    expect(connectionList(state)).toEqual([status({ id: 'a', state: 'REPORTING' })]);
  });

  it('ignores every StreamEvent kind but acquisition, leaving the fold untouched', () => {
    const state = connectionsReducer(initialConnectionsState, {
      type: 'stream', event: { kind: 'connection', connected: true, receivedAtMs: 1 },
    });

    expect(state).toBe(initialConnectionsState);

    const resetState = connectionsReducer(initialConnectionsState, {
      type: 'stream', event: { kind: 'reset', receivedAtMs: 1 },
    });
    expect(resetState).toBe(initialConnectionsState);
  });
});
