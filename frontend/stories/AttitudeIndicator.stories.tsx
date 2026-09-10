import type { Meta, StoryObj } from '@storybook/react-vite';
import { AttitudeIndicator } from '@/ui/AttitudeIndicator';
import type { ReadingState } from '@/ui/readings';
import { reading, stateControl } from './fixtures';

const meta = {
  title: 'Components/Attitude',
  args: { rollDeg: 0, pitchDeg: 0, state: 'live', width: 320 },
  argTypes: {
    rollDeg: { control: { type: 'range', min: -180, max: 180, step: 1 } },
    pitchDeg: { control: { type: 'range', min: -90, max: 90, step: 1 } },
    state: stateControl,
    width: { control: { type: 'range', min: 180, max: 800, step: 10 } },
  },
  render: ({ rollDeg, pitchDeg, state, width }) => <div style={{ width, maxWidth: '100%' }}>
    <AttitudeIndicator reading={reading({ rollDeg, pitchDeg, source: 'ATTITUDE' as const }, state)} />
  </div>,
} satisfies Meta<{ rollDeg: number; pitchDeg: number; state: ReadingState; width: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Level: Story = {};
export const Banked: Story = { args: { rollDeg: 35, pitchDeg: 12 } };
export const Extreme: Story = { args: { rollDeg: -75, pitchDeg: -30 } };
export const Stale: Story = { args: { state: 'stale', rollDeg: 35 } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
