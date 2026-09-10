import { useEffect, useReducer, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { fleetReducer, initialFleetState } from '@/fleet/state';
import { MockEventSource } from '@/stream/mock';
import { VehiclePane } from '@/vehicle/VehiclePane';
import { WorkspaceShell } from '@/workspace/Workspace';
import { sourceLabel } from '@/ui/format';
import { Glance, Lever } from '@/ui/primitives';

import { fixtureFleetOf, NOW } from './fixtures';

interface VehicleArgs {
  fleet: number;
  data: 'frozen' | 'animated' | 'stale' | 'empty';
}

/**
 * The vehicle workspace on its own, in the shell it ships in.
 *
 * `VehiclePane` is the same component the application mounts — Current app
 * imports it too — so anything tuned here is tuned in production. The shell
 * around it is reproduced rather than imported because `FlightDisplay` owns the
 * fleet view as well, and a story that renders both cannot be pointed at one.
 */
function Vehicle({ fleet: size, data }: VehicleArgs) {
  const [fleet, dispatch] = useReducer(
    fleetReducer,
    data === 'empty' || data === 'animated' ? initialFleetState : fixtureFleetOf(size),
  );
  const [nowMs, setNowMs] = useState(
    data === 'animated' ? Date.now : () => NOW + (data === 'stale' ? 6000 : 0),
  );

  useEffect(() => {
    if (data !== 'animated') return;
    const stop = new MockEventSource().start((event) => { dispatch({ type: 'stream', event }); });
    const timer = globalThis.setInterval(() => { setNowMs(Date.now()); }, 250);
    return () => { stop(); globalThis.clearInterval(timer); };
  }, [data]);

  const view = fleet.selected === null ? undefined : fleet.vehicles[fleet.selected];

  return (
    <WorkspaceShell
      meta={<span>{sourceLabel('mock', fleet.connected)}</span>}
      nav={
        <>
          <Lever disabled>← Fleet</Lever>
          {view === undefined ? null : <Glance label="Vehicle" value={view.key} />}
        </>
      }
    >
      <VehiclePane fleet={fleet} view={view} nowMs={nowMs} source="mock" active />
    </WorkspaceShell>
  );
}

const meta = {
  title: 'Frontend/Vehicle pane',
  args: { fleet: 3, data: 'frozen' },
  argTypes: {
    fleet: { control: { type: 'range', min: 1, max: 6, step: 1 } },
    data: { control: 'select', options: ['frozen', 'animated', 'stale', 'empty'] },
  },
  render: (args) => <Vehicle key={`${String(args.fleet)}-${args.data}`} {...args} />,
  parameters: {
    layout: 'fullscreen',
    docs: { description: { component: 'The production VehiclePane in the production shell: the app bar carries navigation and the vehicle picker, the pane carries its own view bar with Views held to the right edge. Public basemap tiles require internet access.' } },
  },
} satisfies Meta<VehicleArgs>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Wide: Story = { globals: { viewport: { value: 'wide' } } };
export const SingleVehicle: Story = { args: { fleet: 1 }, globals: { viewport: { value: 'wide' } } };
export const Narrow: Story = { globals: { viewport: { value: 'narrow' } } };
export const Animated: Story = { args: { data: 'animated' } };
export const Stale: Story = { args: { data: 'stale' } };
export const NoVehicle: Story = { args: { data: 'empty' } };
