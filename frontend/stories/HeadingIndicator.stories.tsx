import type { Meta, StoryObj } from '@storybook/react-vite';
import { HeadingIndicator } from '@/ui/HeadingIndicator';
import type { ReadingState } from '@/ui/readings';
import { reading, stateControl } from './fixtures';

const meta = {
  title: 'Components/Heading',
  args: { headingDeg: 90, fallback: false, state: 'live' },
  argTypes: { headingDeg: { control: { type: 'range', min: 0, max: 359, step: 1 } }, state: stateControl },
  render: ({ headingDeg, fallback, state }) => <div style={{ width: 360, maxWidth: '100%' }}>
    <HeadingIndicator reading={reading({ headingDeg, isFallback: fallback, source: fallback ? 'ATTITUDE' as const : 'VFR_HUD' as const }, state)} />
  </div>,
} satisfies Meta<{ headingDeg: number; fallback: boolean; state: ReadingState }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const East: Story = {};
export const NorthCrossing: Story = { args: { headingDeg: 359 } };
export const Fallback: Story = { args: { fallback: true } };
export const Stale: Story = { args: { state: 'stale' } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
