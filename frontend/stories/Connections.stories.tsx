import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { ConnectionsBar } from '@/connections/ConnectionsBar';
import { FIXTURE_CONNECTION_LIST, FIXTURE_DEVICES, FIXTURE_PROFILES, FIXTURE_STATUSES } from '@/connections/fixtures';
import { emptyConnections } from '@/connections/useConnections';
import type { ConnectionStatus } from '@/connections/types';

/**
 * The app-bar summary and its popover (T-048) — mounted once in
 * `FlightDisplay`'s shared `meta` slot, reachable from an empty fleet and
 * from vehicle detail without a selected vehicle. No backend or SSE stream is
 * involved: every scenario here is one of the fixed §8-state fixtures in
 * `connections/fixtures.ts`.
 */
const meta = {
  title: 'Panels/Connections',
  args: { live: true, connections: FIXTURE_CONNECTION_LIST },
  render: ({ live, connections }) => (
    <div style={{ padding: 8, background: 'var(--panel-deep)' }}>
      <ConnectionsBar
        {...emptyConnections}
        nowMs={Date.parse('2026-09-23T12:00:00Z')}
        live={live}
        connections={connections}
        devices={FIXTURE_DEVICES}
        profiles={FIXTURE_PROFILES}
      />
    </div>
  ),
} satisfies Meta<{ live: boolean; connections: readonly ConnectionStatus[] }>;
export default meta;
type Story = StoryObj<typeof meta>;

/** One bench profile reporting, one field profile awaiting the aircraft — a
 *  plausible steady state. */
export const Summary: Story = {};

export const NothingConfigured: Story = { args: { connections: [] } };

export const NeedsAttention: Story = {
  args: { connections: [FIXTURE_STATUSES.REPORTING, FIXTURE_STATUSES.ACCESS_FAILED] },
};

export const NotLive: Story = {
  args: { live: false },
  name: 'Mock/replay source',
};

/** Opens the popover to show every configured connection's own row, plus the
 *  "Add connection" form — the detailed surface ADR 0006 asks for. */
export const Opened: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('button'));
    await expect(canvas.getByRole('group', { name: 'Connections' })).toBeInTheDocument();
  },
};

/** `AMBIGUOUS`/`DEVICE_MISSING` never auto-resolve — the popover must offer
 *  an explicit device picker instead of a bare Connect action. */
export const NeedsSelection: Story = {
  args: { connections: [FIXTURE_STATUSES.DEVICE_MISSING, FIXTURE_STATUSES.AMBIGUOUS] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('button'));
    await expect(canvas.getAllByText('Select a device…').length).toBeGreaterThan(0);
  },
};
