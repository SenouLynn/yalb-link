import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';
import { MissionPanel, type MissionStatus } from '@/mission/MissionPanel';
import { mockMissionSnapshot } from '@/mission/fixtures';
import { missionGeometry } from '@/mission/model';
import { NOW } from './fixtures';

function missionSnapshot(itemCount: number) {
  const snapshot = mockMissionSnapshot(1, 1, NOW);
  const template = snapshot.items;
  snapshot.items = Array.from({ length: itemCount }, (_, seq) => {
    const item = template[seq % template.length];
    if (!item) throw new Error('Mission fixture is empty');
    return { ...item, seq };
  });
  return snapshot;
}
const meta = {
  title: 'Panels/Mission',
  args: { status: 'complete', itemCount: 6, activeSeq: 1, width: 600, onDownload: fn() },
  argTypes: {
    status: { control: 'select', options: ['idle', 'loading', 'error', 'complete'] },
    itemCount: { control: { type: 'range', min: 0, max: 60, step: 1 } },
    activeSeq: { control: { type: 'number', min: 0 } },
  },
  render: ({ status, itemCount, activeSeq, width, onDownload }) => {
    const snapshot = status === 'complete' ? missionSnapshot(itemCount) : null;
    return <div style={{ width, maxWidth: '100%' }}><MissionPanel status={status} snapshot={snapshot}
      error={status === 'error' ? 'Mission download timed out. Try again.' : null}
      geometry={missionGeometry(snapshot)} activeSeq={activeSeq} onDownload={onDownload} /></div>;
  },
} satisfies Meta<{ status: MissionStatus; itemCount: number; activeSeq: number; width: number; onDownload: () => void }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Populated: Story = {};
export const Idle: Story = { args: { status: 'idle' } };
export const Loading: Story = { args: { status: 'loading' } };
export const DownloadError: Story = { args: { status: 'error' } };
export const Empty: Story = { args: { itemCount: 0 } };
export const LongMission: Story = { args: { itemCount: 40 } };
export const Narrow: Story = { args: { width: 320 }, globals: { viewport: { value: 'narrow' } } };
