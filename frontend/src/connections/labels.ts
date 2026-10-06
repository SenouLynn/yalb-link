/**
 * Operator-facing text and tone for connection state — the one place that
 * spells out the wire's `ConnectionState` strings, so the display and its
 * tests both read the same table instead of two switch statements drifting
 * apart.
 */

import type { Tone } from '@/ui/primitives';

import type { ConnectionState, ConnectionStatus, DeviceInfo, Profile } from './types';

export interface StateInfo {
  /** Distinct per state, by construction of this table (T-048 acceptance). */
  label: string;
  tone: Tone;
}

/**
 * `OPENING`/`OPEN_AWAITING_TRAFFIC`/`REPORTING` are `normal`: all three are
 * ordinary resting states on ADR 0006's own terms (a ground radio waiting for
 * an aircraft to power on is not a fault). `IDLE`/`RELEASED` are `dead`:
 * present but not currently acquiring, the same "nothing to read here" the
 * rest of the display uses `dead` for. Everything else needs operator
 * attention, which is the only thing `caution` is spent on in this system.
 */
const STATE_INFO: Readonly<Record<ConnectionState, StateInfo>> = {
  DEVICE_MISSING: { label: 'DEVICE MISSING', tone: 'caution' },
  AMBIGUOUS: { label: 'AMBIGUOUS', tone: 'caution' },
  IDLE: { label: 'IDLE', tone: 'dead' },
  OPENING: { label: 'OPENING', tone: 'normal' },
  ACCESS_FAILED: { label: 'ACCESS FAILED', tone: 'caution' },
  OPEN_AWAITING_TRAFFIC: { label: 'AWAITING TRAFFIC', tone: 'normal' },
  REPORTING: { label: 'REPORTING', tone: 'normal' },
  INTERRUPTED: { label: 'INTERRUPTED', tone: 'caution' },
  DEVICE_LOST: { label: 'DEVICE LOST', tone: 'caution' },
  TRANSPORT_FAILED: { label: 'TRANSPORT FAILED', tone: 'caution' },
  RELEASED: { label: 'RELEASED', tone: 'dead' },
};

export function stateInfo(state: ConnectionState): StateInfo {
  return STATE_INFO[state];
}

/** `DEVICE_MISSING`/`AMBIGUOUS` never auto-resolve (contract §8) — the
 *  operator must name a device explicitly. */
export function needsSelection(state: ConnectionState): boolean {
  return state === 'DEVICE_MISSING' || state === 'AMBIGUOUS';
}

/** Whether a connect/disconnect action makes sense from this state. */
export function canConnect(state: ConnectionState): boolean {
  return state === 'IDLE' || state === 'RELEASED' || needsSelection(state);
}

export function canDisconnect(state: ConnectionState): boolean {
  return !canConnect(state);
}

/** A saved profile's name when one owns this connection id, else the device's
 *  own identity, falling back to the bare id — never inventing a radio name. */
export function connectionLabel(connectionStatus: ConnectionStatus, profiles: readonly Profile[]): string {
  const profile = profiles.find((candidate) => candidate.id === connectionStatus.id);
  if (profile !== undefined) return profile.name;

  return deviceLabel(connectionStatus.device) ?? connectionStatus.id;
}

export function deviceLabel(device: DeviceInfo | undefined): string | undefined {
  return device?.description ?? device?.serialNumber ?? device?.id;
}

/** The worst-case fact across every connection, for the compact app-bar
 *  summary (ADR 0006: "keep a compact connection summary visible"). Caution
 *  outranks everything, matching how a single caution row anywhere else in
 *  this display draws the eye before any calm one beside it. */
export function summarizeConnections(connections: readonly ConnectionStatus[]): StateInfo {
  if (connections.length === 0) {
    return { label: 'NONE CONFIGURED', tone: 'dead' };
  }

  const attention = connections.find((connection) => stateInfo(connection.state).tone === 'caution');
  if (attention !== undefined) {
    return { label: 'ATTENTION', tone: 'caution' };
  }

  const reporting = connections.filter((connection) => connection.state === 'REPORTING').length;
  if (reporting > 0) {
    return { label: `${String(reporting)}/${String(connections.length)} REPORTING`, tone: 'normal' };
  }

  const active = connections.some((connection) => stateInfo(connection.state).tone === 'normal');
  return active ? { label: 'AWAITING TRAFFIC', tone: 'normal' } : { label: 'IDLE', tone: 'dead' };
}
