import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { WorkspaceShell, WorkspacePanes, PaneControls, DEFAULT_VISIBILITY } from '@/workspace/Workspace';
import { InstrumentPanel } from '@/ui/InstrumentPanel';
import { MissionPanel } from '@/mission/MissionPanel';
import { mockMissionSnapshot } from '@/mission/fixtures';
import { missionGeometry } from '@/mission/model';
import { instrumentReadings, NOW } from './fixtures';

function Workshop() {
  const [visible, setVisible] = useState(DEFAULT_VISIBILITY);
  const [loaded, setLoaded] = useState(true);
  const snapshot = loaded ? mockMissionSnapshot(1, 1, NOW) : null;
  return <WorkspaceShell topbar={<><span className="chip chip--caution">STORY</span>
    <PaneControls visible={visible} onToggle={(id) => { setVisible((current) => ({ ...current, [id]: !current[id] })); }} />
  </>}>
    <aside className="workspace__sidebar"><div className="label">Fixture vehicle · 1:1</div>
      <p>Frozen flight readings</p><button type="button" onClick={() => { setLoaded(false); }}>Clear mission</button>
    </aside>
    <WorkspacePanes visible={visible} panes={{
      instruments: <InstrumentPanel readings={instrumentReadings('live')} />,
      map: <div className="panel" style={{ height: '100%', display: 'grid', placeItems: 'center' }}>Map placeholder · layout preview</div>,
      mission: <MissionPanel status={loaded ? 'complete' : 'idle'} snapshot={snapshot} error={null}
        geometry={missionGeometry(snapshot)} activeSeq={1} onDownload={() => { setLoaded(true); }} />,
    }} />
  </WorkspaceShell>;
}
const meta = { title: 'Composition/Layout shell', component: Workshop, parameters: { layout: 'fullscreen' } } satisfies Meta<typeof Workshop>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Wide: Story = { globals: { viewport: { value: 'wide' } } };
export const Narrow: Story = { globals: { viewport: { value: 'narrow' } } };
