/**
 * The connection fold: acquisition evidence, keyed by connection id.
 *
 * Deliberately separate from `fleet/state.ts`. A connection is an acquisition
 * path, not a vehicle identity (ADR 0006), and the two fail independently: a
 * device can be `REPORTING` while every vehicle on it goes `VEHICLE_LOST`, and
 * reselecting a device must never reset an unrelated vehicle's telemetry. One
 * `StreamEvent` feed drives both folds; nothing here reads `fleet/state.ts` or
 * vice versa.
 */

import type { StreamEvent } from '@/stream/events';

import type { ConnectionStatus } from './types';

export interface ConnectionsState {
  byId: Readonly<Record<string, ConnectionStatus>>;
  /** Stable render order: first-seen, not re-sorted on every update. */
  order: readonly string[];
}

export const initialConnectionsState: ConnectionsState = { byId: {}, order: [] };

export type ConnectionsAction =
  /** The initial `GET /api/connections` read. Replaces the fold outright: a
   *  bootstrap is a full snapshot, never a merge with whatever came before
   *  (a stale entry that bootstrap does not name is no longer configured). */
  | { type: 'bootstrap'; statuses: readonly ConnectionStatus[] }
  | { type: 'stream'; event: StreamEvent };

export function connectionsReducer(state: ConnectionsState, action: ConnectionsAction): ConnectionsState {
  switch (action.type) {
    case 'bootstrap':
      return {
        byId: Object.fromEntries(action.statuses.map((status) => [status.id, status])),
        order: action.statuses.map((status) => status.id),
      };

    case 'stream':
      return applyStreamEvent(state, action.event);

    default:
      return state;
  }
}

function applyStreamEvent(state: ConnectionsState, event: StreamEvent): ConnectionsState {
  switch (event.kind) {
    case 'acquisition':
      return upsert(state, event.event);

    // A replay reset discards accumulated telemetry, but there is no replayed
    // acquisition history to rewind: real serial devices are never part of a
    // recording. Leaving the fold alone is correct, not merely unhandled.
    default:
      return state;
  }
}

function upsert(state: ConnectionsState, status: ConnectionStatus): ConnectionsState {
  const isNew = !(status.id in state.byId);

  return {
    byId: { ...state.byId, [status.id]: status },
    order: isNew ? [...state.order, status.id] : state.order,
  };
}

/** Connections in stable order, for anything that renders the whole list. */
export function connectionList(state: ConnectionsState): ConnectionStatus[] {
  return state.order.map((id) => state.byId[id]).filter((status): status is ConnectionStatus => status !== undefined);
}
