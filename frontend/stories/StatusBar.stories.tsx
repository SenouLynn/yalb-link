import type { Meta, StoryObj } from '@storybook/react-vite';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { LinkRows, StateRows } from '@/ui/StatusBar';
import { Group } from '@/ui/primitives';
import { NOW, vehicle } from './fixtures';

/** Rail width, because that is the only width these rows are ever rendered at. */
const RAIL = 240;

const meta = {
  title: 'Panels/Vehicle status',
  args: { armed: true, lost: false, stale: false, width: RAIL },
  render: ({ armed, lost, stale, width }) => {
    const props = {
      view: {
        ...vehicle,
        lifecycle: lost ? FleetEventType.VEHICLE_LOST : vehicle.lifecycle,
        heartbeat: vehicle.heartbeat ? { ...vehicle.heartbeat, armed } : undefined,
      },
      nowMs: NOW + (stale ? 6000 : 0),
      connected: true,
      source: 'mock' as const,
    };

    return (
      <div style={{ width, maxWidth: '100%' }}>
        <Group label="Link">
          <LinkRows {...props} />
        </Group>
        <Group label="Target state">
          <StateRows {...props} />
        </Group>
      </div>
    );
  },
} satisfies Meta<{ armed: boolean; lost: boolean; stale: boolean; width: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Armed: Story = {};
export const Disarmed: Story = { args: { armed: false } };
export const Lost: Story = { args: { lost: true, stale: true } };
