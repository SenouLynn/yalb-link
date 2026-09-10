import type { Meta, StoryObj } from '@storybook/react-vite';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { StatusBar } from '@/ui/StatusBar';
import { NOW, vehicle } from './fixtures';

const meta = {
  title: 'Panels/Vehicle status', args: { armed: true, lost: false, stale: false, width: 320 },
  render: ({ armed, lost, stale, width }) => <div style={{ width, maxWidth: '100%' }}>
    <StatusBar view={{ ...vehicle, lifecycle: lost ? FleetEventType.VEHICLE_LOST : vehicle.lifecycle,
      heartbeat: vehicle.heartbeat ? { ...vehicle.heartbeat, armed } : undefined }}
      nowMs={NOW + (stale ? 6000 : 0)} connected source="mock" />
  </div>,
} satisfies Meta<{ armed: boolean; lost: boolean; stale: boolean; width: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Armed: Story = {};
export const Disarmed: Story = { args: { armed: false } };
export const Lost: Story = { args: { lost: true, stale: true } };
export const Wide: Story = { args: { width: 1000 } };
