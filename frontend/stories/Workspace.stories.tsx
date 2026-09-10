import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  WorkspaceShell,
  WorkspaceSlots,
  ViewsMenu,
  DEFAULT_VISIBILITY,
  type PanelContent,
  type PanelVisibility,
} from '@/workspace/Workspace';
import { GuidancePanel, HomePanel, RadioLinkPanel } from '@/ui/InspectionPanel';
import { InstrumentPanel } from '@/ui/InstrumentPanel';
import { FamiliesPanel, SamplePanel } from '@/ui/DevPanels';
import { LinkRows, StateRows } from '@/ui/StatusBar';
import { Group, Row } from '@/ui/primitives';
import { MissionPanel } from '@/mission/MissionPanel';
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
        <Row label="Latitude" value="47.393227" />
        <Row label="Longitude" value="8.545423" />
        <Row label="Track" value="160 / 500" unit="pts" />
      </Group>
    ),
    mission: (
      <MissionPanel
        nowMs={NOW}
        status={loaded ? 'complete' : 'idle'}
        snapshot={snapshot}
        error={null}
        geometry={missionGeometry(snapshot)}
        activeSeq={1}
        onDownload={() => {
          setLoaded(true);
        }}
      />
    ),
    map: (
      <div className="slot__empty">Map placeholder · layout preview</div>
    ),
    guidance: <GuidancePanel readings={instrumentReadings('live')} />,
    home: <HomePanel readings={instrumentReadings('live')} />,
    radiolink: <RadioLinkPanel readings={instrumentReadings('live')} />,
    instruments: <InstrumentPanel readings={instrumentReadings('live')} />,
    families: <FamiliesPanel view={vehicle} nowMs={NOW} />,
    sample: <SamplePanel view={vehicle} />,
  };

  return (
    <WorkspaceShell
      title="Ground control"
      meta={<span>STORY</span>}
      viewBar={
        <ViewsMenu
          visible={visible}
          onToggle={(id) => {
            setVisible((current) => ({ ...current, [id]: current[id] !== true }));
          }}
        />
      }
    >
      <WorkspaceSlots visible={visible} content={content} />
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
