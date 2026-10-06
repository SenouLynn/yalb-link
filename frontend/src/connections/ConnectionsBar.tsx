/**
 * The compact connection summary and its popover, mounted once in the app bar
 * (`FlightDisplay`) so it is reachable from an empty fleet and from vehicle
 * detail without depending on a selected vehicle (ADR 0006, T-048).
 *
 * Detailed selection and diagnosis live in the popover, the same
 * disclosure-on-demand shape `ViewsMenu` already established in
 * `workspace/Workspace.tsx` — a second implementation of that shape rather
 * than a shared export, since the two popovers hold unrelated content and
 * `ViewsMenu` is not itself exported as reusable chrome.
 */

import { useEffect, useRef, useState } from 'react';

import { age } from '@/ui/format';
import { Field, Glance, Lever, Note, Row } from '@/ui/primitives';

import {
  canConnect,
  canDisconnect,
  connectionLabel,
  deviceLabel,
  needsSelection,
  stateInfo,
  summarizeConnections,
} from './labels';
import type { UseConnectionsResult } from './useConnections';
import type { ConnectionStatus, DeviceInfo, Profile } from './types';

const DEFAULT_BAUD = 57600;

export interface ConnectionsBarProps extends UseConnectionsResult {
  nowMs: number;
  /** Mock/replay have no backend serial device to manage (`useConnections`'s
   *  own gate); the popover explains this instead of showing dead controls. */
  live: boolean;
}

export function ConnectionsBar(props: ConnectionsBarProps) {
  const { connections, live } = props;
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event: PointerEvent) => {
      if (root.current !== null && !root.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const { refreshDevices, refreshProfiles } = props;
  useEffect(() => {
    if (open && live) {
      refreshDevices();
      refreshProfiles();
    }
  }, [open, live, refreshDevices, refreshProfiles]);

  const summary = summarizeConnections(connections);

  return (
    <div className="connections" ref={root}>
      <Lever
        aria-expanded={open}
        aria-haspopup="true"
        aria-label={`Connections: ${summary.label}`}
        onClick={() => { setOpen((previous) => !previous); }}
      >
        <Glance label="Conn" value={summary.label} tone={summary.tone} />
      </Lever>

      {open ? (
        <div className="connections__popover" role="group" aria-label="Connections">
          {!live ? (
            <Note tone="absent">
              Connection management needs the live backend. This page is reading {' '}
              {props.connections.length === 0 ? 'a fixture or recording' : 'a non-live source'}, which has no serial device to acquire.
            </Note>
          ) : (
            <LiveConnections {...props} />
          )}
        </div>
      ) : null}
    </div>
  );
}

function LiveConnections(props: ConnectionsBarProps) {
  const { connections, devices, profiles, devicesError, profilesError, actionError, pending, nowMs } = props;

  return (
    <>
      {actionError === null ? null : <Note tone="caution" role="alert">{actionError}</Note>}

      {connections.length === 0 ? (
        <Note tone="absent">No connections configured yet. Add one below.</Note>
      ) : (
        connections.map((connection) => (
          <ConnectionRow
            key={connection.id}
            connection={connection}
            devices={devices}
            profiles={profiles}
            busy={pending.has(connection.id)}
            nowMs={nowMs}
            onConnectProfile={props.connectProfile}
            onConnectDevice={props.connectDevice}
            onDisconnect={props.disconnect}
            onDeleteProfile={profiles.some((profile) => profile.id === connection.id) ? props.deleteProfile : undefined}
          />
        ))
      )}

      {devicesError === null ? null : <Note tone="caution">Devices: {devicesError}</Note>}
      {profilesError === null ? null : <Note tone="caution">Profiles: {profilesError}</Note>}

      <NewConnection
        devices={devices}
        busy={pending.has('new-profile')}
        onConnect={props.connectDevice}
        onSave={props.saveProfile}
        onRefreshDevices={props.refreshDevices}
      />
    </>
  );
}

function ConnectionRow({
  connection,
  devices,
  profiles,
  busy,
  nowMs,
  onConnectProfile,
  onConnectDevice,
  onDisconnect,
  onDeleteProfile,
}: {
  connection: ConnectionStatus;
  devices: readonly DeviceInfo[];
  profiles: readonly Profile[];
  busy: boolean;
  nowMs: number;
  onConnectProfile: (profileId: string, deviceId?: string) => void;
  onConnectDevice: (deviceId: string, settings: { baudRate: number }) => void;
  onDisconnect: (id: string) => void;
  onDeleteProfile?: ((id: string) => void) | undefined;
}) {
  const info = stateInfo(connection.state);
  const [pickedDevice, setPickedDevice] = useState('');
  const isProfile = profiles.some((profile) => profile.id === connection.id);

  const frameAge = connection.lastFrameAtMs === undefined ? null : nowMs - connection.lastFrameAtMs;

  return (
    <div className="connections__row" aria-label={connectionLabel(connection, profiles)}>
      <Row
        label={connectionLabel(connection, profiles)}
        value={info.label}
        tone={info.tone}
        hint={deviceLabel(connection.device)}
      />

      {connection.state === 'REPORTING' || connection.state === 'INTERRUPTED' ? (
        <Row label="Last frame" value={frameAge === null ? '—' : age(frameAge)} />
      ) : null}

      {connection.detailedError === undefined ? null : (
        <Note tone="caution">{connection.detailedError}</Note>
      )}

      {needsSelection(connection.state) ? (
        <div className="connections__pick">
          <Field
            label="Device"
            value={pickedDevice}
            onChange={setPickedDevice}
            options={[{ value: '', label: 'Select a device…' }, ...devices.map((device) => ({
              value: device.id,
              label: deviceLabel(device) ?? device.id,
            }))]}
          />
          <Lever
            disabled={busy || pickedDevice === ''}
            onClick={() => { onConnectProfile(connection.id, pickedDevice); }}
          >
            Select
          </Lever>
        </div>
      ) : (
        <div className="lever-row">
          {canConnect(connection.state) ? (
            <Lever
              disabled={busy}
              onClick={() => {
                if (isProfile) onConnectProfile(connection.id);
                else if (connection.device !== undefined) {
                  onConnectDevice(connection.device.id, connection.settings ?? { baudRate: DEFAULT_BAUD });
                }
              }}
            >
              Connect
            </Lever>
          ) : null}
          {canDisconnect(connection.state) ? (
            <Lever disabled={busy} onClick={() => { onDisconnect(connection.id); }}>
              Disconnect
            </Lever>
          ) : null}
          {onDeleteProfile === undefined ? null : (
            <Lever disabled={busy} onClick={() => { onDeleteProfile(connection.id); }}>
              Delete profile
            </Lever>
          )}
        </div>
      )}
    </div>
  );
}

function NewConnection({
  devices,
  busy,
  onConnect,
  onSave,
  onRefreshDevices,
}: {
  devices: readonly DeviceInfo[];
  busy: boolean;
  onConnect: (deviceId: string, settings: { baudRate: number }) => void;
  onSave: (profile: { name: string; deviceId: string; settings: { baudRate: number } }) => void;
  onRefreshDevices: () => void;
}) {
  const [deviceId, setDeviceId] = useState('');
  const [baud, setBaud] = useState(String(DEFAULT_BAUD));
  const [name, setName] = useState('');

  const settings = { baudRate: Number.parseInt(baud, 10) || DEFAULT_BAUD };
  const canAct = deviceId !== '' && !busy;

  return (
    <div className="connections__new">
      <div className="group__head">
        <span className="group__label">Add connection</span>
        <Lever compact quiet aria-label="Refresh devices" title="Refresh devices" onClick={onRefreshDevices}>
          ↻
        </Lever>
      </div>

      {devices.length === 0 ? (
        <Note tone="absent">No devices found. Attach one, then refresh.</Note>
      ) : (
        <>
          <Field
            label="Device"
            value={deviceId}
            onChange={setDeviceId}
            options={[{ value: '', label: 'Select a device…' }, ...devices.map((device) => ({
              value: device.id,
              label: deviceLabel(device) ?? device.id,
            }))]}
          />
          <Field label="Baud" value={baud} onChange={setBaud} inputMode="numeric" />
          <Field label="Save as" value={name} onChange={setName} placeholder="Optional profile name" />

          <div className="lever-row">
            <Lever disabled={!canAct} onClick={() => { onConnect(deviceId, settings); }}>
              Connect
            </Lever>
            <Lever
              disabled={!canAct || name.trim() === ''}
              onClick={() => { onSave({ name: name.trim(), deviceId, settings }); }}
            >
              Save &amp; connect
            </Lever>
          </div>
        </>
      )}
    </div>
  );
}
