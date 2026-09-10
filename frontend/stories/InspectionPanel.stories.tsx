import type { Meta, StoryObj } from '@storybook/react-vite';
import { InspectionPanel } from '@/ui/InspectionPanel';
import type { ReadingState } from '@/ui/readings';
import { instrumentReadings, stateControl } from './fixtures';

const meta = {
  title: 'Panels/Inspection', args: { state: 'live', width: 600 },
  argTypes: { state: stateControl },
  render: ({ state, width }) => <div style={{ width, maxWidth: '100%' }}><InspectionPanel readings={instrumentReadings(state)} /></div>,
} satisfies Meta<{ state: ReadingState; width: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Normal: Story = {};
export const Stale: Story = { args: { state: 'stale' } };
export const Unavailable: Story = { args: { state: 'unavailable' } };
export const Narrow: Story = { args: { width: 320 }, globals: { viewport: { value: 'narrow' } } };
