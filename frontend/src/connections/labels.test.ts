import { describe, expect, it } from 'vitest';

import { FIXTURE_PROFILES, FIXTURE_STATUSES } from './fixtures';
import {
  canConnect,
  canDisconnect,
  connectionLabel,
  needsSelection,
  stateInfo,
  summarizeConnections,
} from './labels';
import type { ConnectionState } from './types';

const ALL_STATES = Object.keys(FIXTURE_STATUSES) as ConnectionState[];

describe('stateInfo', () => {
  it('gives every state a distinct operator-visible label (T-048 acceptance)', () => {
    const labels = ALL_STATES.map((state) => stateInfo(state).label);
    expect(new Set(labels).size).toBe(ALL_STATES.length);
  });

  it('never uses "good" tone — the display has no such tone', () => {
    for (const state of ALL_STATES) {
      expect(['normal', 'caution', 'dead']).toContain(stateInfo(state).tone);
    }
  });

  it('renders OPEN_AWAITING_TRAFFIC as a normal resting state, not a warning', () => {
    expect(stateInfo('OPEN_AWAITING_TRAFFIC').tone).toBe('normal');
  });

  it('flags every state needing operator attention as caution', () => {
    const attention: ConnectionState[] = [
      'DEVICE_MISSING', 'AMBIGUOUS', 'ACCESS_FAILED', 'INTERRUPTED', 'DEVICE_LOST', 'TRANSPORT_FAILED',
    ];
    for (const state of attention) {
      expect(stateInfo(state).tone).toBe('caution');
    }
  });
});

describe('needsSelection / canConnect / canDisconnect', () => {
  it('only DEVICE_MISSING and AMBIGUOUS need explicit selection', () => {
    for (const state of ALL_STATES) {
      expect(needsSelection(state)).toBe(state === 'DEVICE_MISSING' || state === 'AMBIGUOUS');
    }
  });

  it('every state offers exactly one of connect or disconnect, never both or neither', () => {
    for (const state of ALL_STATES) {
      expect(canConnect(state)).toBe(!canDisconnect(state));
    }
  });
});

describe('connectionLabel', () => {
  it('prefers a saved profile\'s name over the device identity', () => {
    const profile = FIXTURE_PROFILES.find((p) => p.id === 'bench-controller');
    expect(profile).toBeDefined();
    expect(connectionLabel(FIXTURE_STATUSES.REPORTING, FIXTURE_PROFILES)).toBe('Bench controller');
  });

  it('falls back to the device identity, then the bare id, for an ad hoc connection', () => {
    expect(connectionLabel(FIXTURE_STATUSES.REPORTING, [])).toBe('CubeOrange');
    expect(connectionLabel({ id: 'raw-id', state: 'IDLE', vehicleKeys: [] }, [])).toBe('raw-id');
  });
});

describe('summarizeConnections', () => {
  it('reports nothing configured for an empty fold', () => {
    expect(summarizeConnections([])).toEqual({ label: 'NONE CONFIGURED', tone: 'dead' });
  });

  it('outranks a reporting connection with one needing attention', () => {
    const summary = summarizeConnections([FIXTURE_STATUSES.REPORTING, FIXTURE_STATUSES.ACCESS_FAILED]);
    expect(summary.tone).toBe('caution');
  });

  it('counts reporting connections against the total when nothing needs attention', () => {
    const summary = summarizeConnections([FIXTURE_STATUSES.REPORTING, FIXTURE_STATUSES.OPEN_AWAITING_TRAFFIC]);
    expect(summary).toEqual({ label: '1/2 REPORTING', tone: 'normal' });
  });
});
