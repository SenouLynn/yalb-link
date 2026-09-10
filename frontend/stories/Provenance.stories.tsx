import type { Meta, StoryObj } from '@storybook/react-vite';
import { Provenance } from '@/ui/Provenance';
import type { ReadingState } from '@/ui/readings';
import { reading, stateControl } from './fixtures';

const meta = {
  title: 'Components/Provenance',
  args: { state: 'live', ageMs: 200, source: 'ATTITUDE' },
  argTypes: { state: stateControl, ageMs: { control: { type: 'range', min: 0, max: 10000, step: 100 } } },
  render: ({ state, ageMs, source }) => <div className="panel" style={{ width: 320, maxWidth: '100%' }}>
    <Provenance reading={{ ...reading({ source }, state), ageMs: state === 'unavailable' ? null : ageMs }} />
  </div>,
} satisfies Meta<{ state: ReadingState; ageMs: number; source: string }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Fresh: Story = {};
export const Aging: Story = { args: { ageMs: 4200 } };
export const Stale: Story = { args: { state: 'stale', ageMs: 6000 } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
