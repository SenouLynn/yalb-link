import { useState, type ReactNode } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  ViewBar,
  WorkspaceShell,
  WorkspaceSlots,
  ViewsMenu,
  DEFAULT_MIRRORED,
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

/** A minimal stand-in for `MirrorLever` — the rail's "+" that mirrors a panel
 *  back onto the map or beside the instruments without touching the
 *  accordion it sits inside. */
function MirrorLever({
  id,
  mirrored,
  onToggle,
}: {
  id: string;
  mirrored: boolean;
  onToggle: (id: string) => void;
}) {
  return (
    <Lever
      compact
      pressed={mirrored}
      aria-label={mirrored ? `Stop mirroring ${id}` : `Mirror ${id}`}
      onClick={(event) => {
        event.preventDefault();
        onToggle(id);
      }}
    >
      +
    </Lever>
  );
}

/**
 * The shell with every registered panel mounted.
 *
 * This is the story that catches slot regressions: toggling panels off in Views
 * must reflow without leaving a gap, and no slot may push the page sideways.
 * It also has to demonstrate the accordion/mirror behavior the real
 * `VehiclePane` wires up, or it stops catching regressions in that instead.
 */
function Workshop() {
  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);
  const [mirrored, setMirrored] = useState<PanelVisibility>(DEFAULT_MIRRORED);
  const [loaded, setLoaded] = useState(true);
  const snapshot = loaded ? mockMissionSnapshot(1, 1, NOW) : null;
  const status = { view: vehicle, nowMs: NOW, connected: true, source: 'mock' as const };
  const readings = instrumentReadings('live');

  const toggleMirror = (id: string) => {
    setMirrored((current) => ({ ...current, [id]: current[id] !== true }));
  };

  const position = (collapsible?: boolean, actions?: ReactNode) => (
    <Group label="Position" reading={readings.position} collapsible={collapsible} actions={actions}>
      <Row label="Lat / lon" value="47.393227, 8.545423" />
      <Row label="Track" value="160 / 500" unit="pts" />
      <HomeRows readings={readings} />
    </Group>
  );

  const content: PanelContent = {
    link: <LinkRows {...status} collapsible />,
    state: <StateRows {...status} collapsible />,
    position: position(
      true,
      <MirrorLever id="position" mirrored={mirrored['position'] === true} onToggle={toggleMirror} />,
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
        collapsible
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
    guidance: (
      <GuidancePanel
        readings={readings}
        collapsible
        actions={<MirrorLever id="guidance" mirrored={mirrored['guidance'] === true} onToggle={toggleMirror} />}
      />
    ),
    radiolink: (
      <RadioLinkPanel
        readings={readings}
        collapsible
        actions={<MirrorLever id="radiolink" mirrored={mirrored['radiolink'] === true} onToggle={toggleMirror} />}
      />
    ),
    instruments: <InstrumentPanel readings={readings} />,
    families: <FamiliesPanel view={vehicle} nowMs={NOW} />,
    sample: <SamplePanel view={vehicle} />,
  };

  const mirrorContent: PanelContent = {
    position: position(),
    guidance: <GuidancePanel readings={readings} />,
    radiolink: <RadioLinkPanel readings={readings} />,
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
        <WorkspaceSlots
          visible={visible}
          mirrored={mirrored}
          content={content}
          mirrorContent={mirrorContent}
          sectionHeaders={{
            map: (
              <div className="critical-bar">
                <Group label="Command"><Row label="Arm" value="DISARMED" /></Group>
              </div>
            ),
          }}
        />
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
