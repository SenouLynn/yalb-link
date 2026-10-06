// @vitest-environment happy-dom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ConnectionsBar, type ConnectionsBarProps } from './ConnectionsBar';
import { FIXTURE_CONNECTION_LIST, FIXTURE_DEVICES, FIXTURE_PROFILES, FIXTURE_STATUSES } from './fixtures';
import { emptyConnections } from './useConnections';

const FIELD_RADIO_DEVICE = FIXTURE_DEVICES[0];
if (FIELD_RADIO_DEVICE === undefined) throw new Error('fixture setup: no devices');

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  if (root !== null) act(() => { root?.unmount(); });
  container?.remove();
  container = null;
  root = null;
});

function render(props: Partial<ConnectionsBarProps> = {}) {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  const full: ConnectionsBarProps = {
    ...emptyConnections,
    nowMs: Date.parse('2026-09-23T12:00:00Z'),
    live: true,
    ...props,
  };
  act(() => { root?.render(<ConnectionsBar {...full} />); });
  return container;
}

function openPopover(el: HTMLElement) {
  const trigger = el.querySelector<HTMLButtonElement>('.connections > .lever');
  act(() => { trigger?.click(); });
}

describe('ConnectionsBar', () => {
  it('summarizes an empty fold and no connections configured', () => {
    const el = render();
    expect(el.textContent).toContain('NONE CONFIGURED');
  });

  it('summarizes a mix of connections, caution outranking a reporting one', () => {
    const el = render({ connections: [FIXTURE_STATUSES.REPORTING, FIXTURE_STATUSES.ACCESS_FAILED] });
    expect(el.textContent).toContain('ATTENTION');
  });

  it('is reachable and openable without any discovered vehicle — the trigger needs no vehicle prop at all', () => {
    const el = render({ connections: [] });
    expect(el.querySelector('[aria-haspopup="true"]')).not.toBeNull();
    openPopover(el);
    expect(el.querySelector('.connections__popover')).not.toBeNull();
  });

  it('closes on Escape', () => {
    const el = render();
    openPopover(el);
    expect(el.querySelector('.connections__popover')).not.toBeNull();
    act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); });
    expect(el.querySelector('.connections__popover')).toBeNull();
  });

  it('closes on an outside pointerdown', () => {
    const el = render();
    openPopover(el);
    act(() => { document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })); });
    expect(el.querySelector('.connections__popover')).toBeNull();
  });

  it('explains that mock/replay sources have no backend device to manage, and shows no live controls', () => {
    const el = render({ live: false, connections: FIXTURE_CONNECTION_LIST });
    openPopover(el);
    expect(el.textContent).toContain('needs the live backend');
    expect(el.querySelector('.connections__new')).toBeNull();
  });

  it('renders every configured connection with its own label and a distinct state', () => {
    const el = render({ connections: FIXTURE_CONNECTION_LIST, profiles: FIXTURE_PROFILES, devices: FIXTURE_DEVICES });
    openPopover(el);
    expect(el.textContent).toContain('Bench controller');
    expect(el.textContent).toContain('REPORTING');
    expect(el.textContent).toContain('Field radio');
    expect(el.textContent).toContain('AWAITING TRAFFIC');
  });

  it('disconnects a reporting connection by id', () => {
    const onDisconnect = vi.fn();
    const el = render({ connections: [FIXTURE_STATUSES.REPORTING], profiles: FIXTURE_PROFILES, disconnect: onDisconnect });
    openPopover(el);
    const disconnectButton = Array.from(el.querySelectorAll('button')).find((b) => b.textContent === 'Disconnect');
    act(() => { disconnectButton?.click(); });
    expect(onDisconnect).toHaveBeenCalledWith('bench-controller');
  });

  it('connects an idle connection by profile id', () => {
    const onConnectProfile = vi.fn();
    const el = render({
      connections: [{ id: 'bench-controller', state: 'IDLE', vehicleKeys: [] }],
      profiles: FIXTURE_PROFILES,
      connectProfile: onConnectProfile,
    });
    openPopover(el);
    const connectButton = Array.from(el.querySelectorAll('button')).find((b) => b.textContent === 'Connect');
    act(() => { connectButton?.click(); });
    expect(onConnectProfile).toHaveBeenCalledWith('bench-controller');
  });

  it('AMBIGUOUS presents an explicit device picker rather than auto-resolving', () => {
    const onConnectProfile = vi.fn();
    const el = render({
      connections: [FIXTURE_STATUSES.AMBIGUOUS],
      profiles: FIXTURE_PROFILES,
      devices: FIXTURE_DEVICES,
      connectProfile: onConnectProfile,
    });
    openPopover(el);

    // No bare Connect lever on the connection's own row: selection is
    // required first. ("Connect" still exists below, in the unrelated "Add
    // connection" form for an ad hoc device.)
    const row = el.querySelector('.connections__row');
    expect(Array.from(row?.querySelectorAll('button') ?? []).some((b) => b.textContent === 'Connect')).toBe(false);

    const select = el.querySelector<HTMLSelectElement>('.connections__pick select');
    expect(select).not.toBeNull();
    act(() => {
      if (select) {
        select.value = FIELD_RADIO_DEVICE.id;
        select.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });
    const selectButton = Array.from(el.querySelectorAll('button')).find((b) => b.textContent === 'Select');
    act(() => { selectButton?.click(); });

    expect(onConnectProfile).toHaveBeenCalledWith('field-radio', FIELD_RADIO_DEVICE.id);
  });

  it('renders the backend\'s own error text verbatim for a failed connection', () => {
    const el = render({ connections: [FIXTURE_STATUSES.ACCESS_FAILED] });
    openPopover(el);
    expect(el.textContent).toContain('Permission denied');
  });

  it('surfaces the last action error without inventing a cause', () => {
    const el = render({ actionError: 'device or resource busy' });
    openPopover(el);
    expect(el.textContent).toContain('device or resource busy');
  });

  it('connects a freshly picked device with its chosen baud rate', () => {
    const onConnectDevice = vi.fn();
    const el = render({ devices: FIXTURE_DEVICES, connectDevice: onConnectDevice });
    openPopover(el);

    const selects = el.querySelectorAll<HTMLSelectElement>('.connections__new select');
    const deviceSelect = selects[0];
    act(() => {
      if (deviceSelect) {
        deviceSelect.value = FIELD_RADIO_DEVICE.id;
        deviceSelect.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });
    const connectButton = Array.from(el.querySelectorAll<HTMLButtonElement>('.connections__new button'))
      .find((b) => b.textContent === 'Connect');
    act(() => { connectButton?.click(); });

    expect(onConnectDevice).toHaveBeenCalledWith(FIELD_RADIO_DEVICE.id, { baudRate: 57600 });
  });

  it('refreshes devices and profiles when the popover opens, not on every render', () => {
    const refreshDevices = vi.fn();
    const refreshProfiles = vi.fn();
    const el = render({ refreshDevices, refreshProfiles });
    expect(refreshDevices).not.toHaveBeenCalled();
    openPopover(el);
    expect(refreshDevices).toHaveBeenCalledTimes(1);
    expect(refreshProfiles).toHaveBeenCalledTimes(1);
  });
});
