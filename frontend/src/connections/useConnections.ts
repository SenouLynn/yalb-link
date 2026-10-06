/**
 * Owns the connection fold, the device/profile inventories, and the actions
 * that mutate them — the one place `ConnectionsBar` and the app shell both
 * talk to.
 *
 * Device and profile inventories are not part of the SSE bootstrap (only
 * `Status` is, per contract §6): they are fetched on demand, mirroring
 * `VehiclePane`'s own on-demand mission download rather than a poll loop the
 * operator never asked for.
 */

import { useCallback, useEffect, useState } from 'react';

import type { StreamEvent } from '@/stream/events';

import {
  connect as apiConnect,
  disconnect as apiDisconnect,
  deleteProfile as apiDeleteProfile,
  listConnections,
  listDevices,
  listProfiles,
  saveProfile as apiSaveProfile,
  ConnectionHTTPError,
} from './api';
import { connectionList, connectionsReducer, initialConnectionsState, type ConnectionsState } from './state';
import type { ConnectionSettings, ConnectionStatus, DeviceInfo, Profile } from './types';

export interface UseConnectionsResult {
  connections: readonly ConnectionStatus[];
  devices: readonly DeviceInfo[];
  profiles: readonly Profile[];
  devicesError: string | null;
  profilesError: string | null;
  /** The most recent action's failure, cleared by the next attempt. */
  actionError: string | null;
  /** Connection/profile ids with a request in flight, for disabling their
   *  own controls without freezing the rest of the popover. */
  pending: ReadonlySet<string>;
  /** Feeds one backend `StreamEvent` into the fold. Ignores every kind but
   *  `acquisition`; safe to call with all of them. */
  dispatchStream: (event: StreamEvent) => void;
  refreshDevices: () => void;
  refreshProfiles: () => void;
  connectDevice: (deviceId: string, settings: ConnectionSettings) => void;
  connectProfile: (profileId: string, deviceId?: string) => void;
  disconnect: (id: string) => void;
  saveProfile: (profile: { name: string; deviceId: string; settings: ConnectionSettings }) => void;
  deleteProfile: (id: string) => void;
}

/**
 * A `FlightDisplay` fed straight fixtures (most existing tests and stories)
 * has no App-level `useConnections` above it, and does not need one to
 * exercise anything unrelated to this card. `FlightDisplay`'s `connections`
 * prop defaults to this rather than being required everywhere `replay`
 * already isn't.
 */
export const emptyConnections: UseConnectionsResult = {
  connections: [],
  devices: [],
  profiles: [],
  devicesError: null,
  profilesError: null,
  actionError: null,
  pending: new Set(),
  dispatchStream: () => undefined,
  refreshDevices: () => undefined,
  refreshProfiles: () => undefined,
  connectDevice: () => undefined,
  connectProfile: () => undefined,
  disconnect: () => undefined,
  saveProfile: () => undefined,
  deleteProfile: () => undefined,
};

function messageOf(error: unknown): string {
  return error instanceof ConnectionHTTPError || error instanceof Error
    ? error.message
    : 'The request failed.';
}

/**
 * `live` gates every network call. Mock and replay sources have no backend
 * serial device behind them (`stream/mock.ts` is a fixture player with no
 * connection to acquire), and calling through to whatever the dev proxy
 * happens to reach would show real hardware state during a demo.
 */
export function useConnections(live: boolean): UseConnectionsResult {
  const [state, setState] = useState<ConnectionsState>(initialConnectionsState);
  const [devices, setDevices] = useState<DeviceInfo[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [devicesError, setDevicesError] = useState<string | null>(null);
  const [profilesError, setProfilesError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pending, setPending] = useState<ReadonlySet<string>>(new Set());

  const dispatchStream = useCallback((event: StreamEvent) => {
    setState((previous) => connectionsReducer(previous, { type: 'stream', event }));
  }, []);

  useEffect(() => {
    if (!live) return;
    listConnections()
      .then((statuses) => { setState(connectionsReducer(initialConnectionsState, { type: 'bootstrap', statuses })); })
      .catch(() => { /* left empty; the acquisition stream still bootstraps per-connection state */ });
  }, [live]);

  const refreshDevices = useCallback(() => {
    if (!live) return;
    listDevices()
      .then((list) => { setDevices(list); setDevicesError(null); })
      .catch((error: unknown) => { setDevicesError(messageOf(error)); });
  }, [live]);

  const refreshProfiles = useCallback(() => {
    if (!live) return;
    listProfiles()
      .then((list) => { setProfiles(list); setProfilesError(null); })
      .catch((error: unknown) => { setProfilesError(messageOf(error)); });
  }, [live]);

  const runAction = useCallback(
    (id: string, task: () => Promise<ConnectionStatus | undefined>) => {
      setActionError(null);
      setPending((previous) => new Set(previous).add(id));
      task()
        .then((status) => {
          // `disconnect` resolves to `undefined` (the backend's own response
          // carries no `Status`, contract §6/`api.ts`); the `acquisition` SSE
          // event this action provokes is what applies `RELEASED`, the same
          // path a change made from another tab or the backend's own retry
          // logic already goes through.
          if (status === undefined) return;
          setState((previous) => connectionsReducer(previous, {
            type: 'stream',
            event: { kind: 'acquisition', event: status, receivedAtMs: Date.now() },
          }));
        })
        .catch((error: unknown) => { setActionError(messageOf(error)); })
        .finally(() => {
          setPending((previous) => {
            const next = new Set(previous);
            next.delete(id);
            return next;
          });
        });
    },
    [],
  );

  const connectDevice = useCallback(
    (deviceId: string, settings: ConnectionSettings) => {
      runAction(deviceId, () => apiConnect({ deviceId, settings }));
    },
    [runAction],
  );

  const connectProfile = useCallback(
    (profileId: string, deviceId?: string) => {
      runAction(profileId, () => apiConnect({ profileId, deviceId }));
    },
    [runAction],
  );

  const disconnect = useCallback(
    (id: string) => {
      runAction(id, async () => { await apiDisconnect(id); return undefined; });
    },
    [runAction],
  );

  const saveProfile = useCallback(
    (profile: { name: string; deviceId: string; settings: ConnectionSettings }) => {
      setActionError(null);
      setPending((previous) => new Set(previous).add('new-profile'));
      apiSaveProfile(profile)
        .then((saved) => {
          setProfiles((previous) => [...previous.filter((p) => p.id !== saved.id), saved]);
          connectProfile(saved.id);
        })
        .catch((error: unknown) => { setActionError(messageOf(error)); })
        .finally(() => {
          setPending((previous) => {
            const next = new Set(previous);
            next.delete('new-profile');
            return next;
          });
        });
    },
    [connectProfile],
  );

  const deleteProfile = useCallback((id: string) => {
    setActionError(null);
    setPending((previous) => new Set(previous).add(id));
    apiDeleteProfile(id)
      .then(() => { setProfiles((previous) => previous.filter((profile) => profile.id !== id)); })
      .catch((error: unknown) => { setActionError(messageOf(error)); })
      .finally(() => {
        setPending((previous) => {
          const next = new Set(previous);
          next.delete(id);
          return next;
        });
      });
  }, []);

  return {
    connections: connectionList(state),
    devices,
    profiles,
    devicesError,
    profilesError,
    actionError,
    pending,
    dispatchStream,
    refreshDevices,
    refreshProfiles,
    connectDevice,
    connectProfile,
    disconnect,
    saveProfile,
    deleteProfile,
  };
}
