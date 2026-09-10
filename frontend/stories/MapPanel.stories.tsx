import type { Meta, StoryObj } from '@storybook/react-vite';
import { MapPanel } from '@/map/MapPanel';
import { mockMissionSnapshot } from '@/mission/fixtures';
import { missionGeometry } from '@/mission/model';
import { hasDisplayValue } from '@/ui/readings';
import { instrumentReadings, NOW, vehicle } from './fixtures';

const snapshot = mockMissionSnapshot(1, 1, NOW);
const position = instrumentReadings('live').position;
const meta = {
  title: 'Panels/Map', args: { showPosition: true, showMission: true, height: 600 },
  render: ({ showPosition, showMission, height }) => <div className="pane--map" style={{ height }}>
    <MapPanel position={showPosition && hasDisplayValue(position) ? position.value : null}
      track={showPosition ? vehicle.track : []} vehicleKey={vehicle.key}
      mission={missionGeometry(showMission ? snapshot : null)} missionRevision={showMission ? snapshot : null} />
  </div>,
  parameters: { docs: { description: { component: 'Real MapLibre panel. Basemap imagery uses the same public tile services as the app and requires internet access.' } } },
} satisfies Meta<{ showPosition: boolean; showMission: boolean; height: number }>;
export default meta;
type Story = StoryObj<typeof meta>;
export const MissionAndPosition: Story = {};
export const PositionOnly: Story = { args: { showMission: false } };
export const NoPosition: Story = { args: { showPosition: false, showMission: false } };
export const Narrow: Story = { globals: { viewport: { value: 'narrow' } } };
