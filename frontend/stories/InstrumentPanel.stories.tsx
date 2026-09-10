import type { Meta, StoryObj } from '@storybook/react-vite';
import { InstrumentPanel } from '@/ui/InstrumentPanel';
import type { ReadingState } from '@/ui/readings';
import { instrumentReadings, stateControl } from './fixtures';

const meta = {
  title: 'Panels/Instruments',
  args: { state: 'live', width: 640 },
  argTypes: { state: stateControl, width: { control: { type: 'range', min: 280, max: 1200, step: 10 } } },
  render: ({ state, width }) => <div style={{ width, maxWidth: '100%' }}><InstrumentPanel readings={instrumentReadings(state)} /></div>,
} satisfies Meta<{ state: ReadingState; width: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Normal: Story = {};
export const Narrow: Story = { args: { width: 320 }, globals: { viewport: { value: 'narrow' } } };
export const Stale: Story = { args: { state: 'stale' } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
