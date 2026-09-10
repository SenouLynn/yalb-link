import type { Meta, StoryObj } from '@storybook/react-vite';
import { Readout } from '@/ui/Readout';
import { NO_VALUE } from '@/ui/format';
import type { ReadingState } from '@/ui/readings';
import { reading, stateControl } from './fixtures';

const meta = {
  title: 'Components/Readout',
  args: { label: 'Altitude', value: '132.5', unit: 'm', note: 'above sea level', state: 'live' },
  argTypes: { state: stateControl },
  render: ({ state, ...args }) => <div style={{ width: 320, maxWidth: '100%' }}><Readout {...args}
    value={state === 'live' ? args.value : NO_VALUE}
    reading={reading({ source: 'GLOBAL_POSITION_INT' }, state)} /></div>,
} satisfies Meta<{ label: string; value: string; unit: string; note: string; state: ReadingState }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Altitude: Story = {};
export const LargeValue: Story = { args: { value: '12345.6' } };
export const Stale: Story = { args: { state: 'stale' } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
