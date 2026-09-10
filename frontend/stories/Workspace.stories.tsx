import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  ViewBar,
  WorkspaceShell,
  WorkspaceSlots,
  ViewsMenu,
  DEFAULT_VISIBILITY,
  toggleSection,
  type PanelContent,
  type PanelVisibility,
} from '@/workspace/Workspace';
import { GuidancePanel, HomeRows, RadioLinkPanel } from '@/ui/InspectionPanel';
import { InstrumentPanel } from '@/ui/InstrumentPanel';
import { FamiliesPanel, SamplePanel } from '@/ui/DevPanels';
import { LinkRows, StateRows } from '@/ui/StatusBar';
import { Group, Lever, Row } from '@/ui/primitives';
import { MissionPanel, WaypointList } from '@/mission/MissionPanel';
import { mockMissionSnapshot } from '@/mission/fixtures';
import { missionGeometry } from '@/mission/model';
import { instrumentReadings, NOW, vehicle } from './fixtures';

/**
 * The shell with every registered panel mounted.
 *
 * This is the story that catches slot regressions: toggling panels off in Views
 * must reflow without leaving a gap, and no slot may push the page sideways.
 */
function Workshop() {
  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);
  const [loaded, setLoaded] = useState(true);
  const snapshot = loaded ? mockMissionSnapshot(1, 1, NOW) : null;
  const status = { view: vehicle, nowMs: NOW, connected: true, source: 'mock' as const };

  const content: PanelContent = {
    link: <LinkRows {...status} />,
    state: <StateRows {...status} />,
    command: <Group label="Command"><Row label="Arm" value="DISARMED" /></Group>,
    position: (
      <Group label="Position" reading={instrumentReadings('live').position}>
        <Row label="Lat / lon" value="47.393227, 8.545423" />
        <Row label="Track" value="160 / 500" unit="pts" />
        <HomeRows readings={instrumentReadings('live')} />
      </Group>
    ),
    mission: (
      <MissionPanel
        nowMs={NOW}
        status={loaded ? 'complete' : 'idle'}
        snapshot={snapshot}
        error={null}
        activeSeq={1}
        onDownload={() => {
          setLoaded(true);
        }}
      />
    ),
    waypoints: (
      <WaypointList
        nowMs={NOW}
        snapshot={snapshot}
        geometry={missionGeometry(snapshot)}
        activeSeq={1}
      />
    ),
    map: (
      <div className="slot__empty">Map placeholder · layout preview</div>
    ),
    guidance: <GuidancePanel readings={instrumentReadings('live')} />,
    radiolink: <RadioLinkPanel readings={instrumentReadings('live')} />,
    instruments: <InstrumentPanel readings={instrumentReadings('live')} />,
    families: <FamiliesPanel view={vehicle} nowMs={NOW} />,
    sample: <SamplePanel view={vehicle} />,
  };

  return (
    <WorkspaceShell meta={<span>STORY</span>} nav={<Lever disabled>← Fleet</Lever>}>
      <div className="view">
        <ViewBar
          trailing={
            <ViewsMenu
              visible={visible}
              onToggle={(id) => {
                setVisible((current) => ({ ...current, [id]: current[id] !== true }));
              }}
              onToggleSection={(id) => {
                setVisible((current) => toggleSection(id, current));
              }}
            />
          }
        />
        <WorkspaceSlots visible={visible} content={content} />
      </div>
    </WorkspaceShell>
  );
}

const meta = {
  title: 'Composition/Layout shell',
  component: Workshop,
  parameters: { layout: 'fullscreen' },
} satisfies Meta<typeof Workshop>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Wide: Story = { globals: { viewport: { value: 'wide' } } };
export const Narrow: Story = { globals: { viewport: { value: 'narrow' } } };
