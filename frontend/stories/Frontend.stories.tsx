import { useEffect, useReducer, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { fleetReducer, initialFleetState } from '@/fleet/state';
import { MockEventSource } from '@/stream/mock';
import { FlightDisplay } from '@/ui/FlightDisplay';
import { fixtureFleet, NOW } from './fixtures';

interface FrontendArgs {
  section: 'fleet' | 'vehicle';
  data: 'frozen' | 'animated' | 'stale' | 'empty';
}

/** App's stream/clock wiring with an explicit mock source; all visual composition is production code. */
function Frontend({ section, data }: FrontendArgs) {
  const [fleet, dispatch] = useReducer(fleetReducer, data === 'empty' || data === 'animated' ? initialFleetState : fixtureFleet);
  const [nowMs, setNowMs] = useState(data === 'animated' ? Date.now : () => NOW + (data === 'stale' ? 6000 : 0));
  useEffect(() => {
    if (data !== 'animated') return;
    const stop = new MockEventSource().start((event) => { dispatch({ type: 'stream', event }); });
    const timer = globalThis.setInterval(() => { setNowMs(Date.now()); }, 250);
    return () => { stop(); globalThis.clearInterval(timer); };
  }, [data]);
  return <FlightDisplay fleet={fleet} nowMs={nowMs} source="mock" initialSection={section}
    onSelect={(key) => { dispatch({ type: 'select', key }); }} />;
}

const meta = {
  title: 'Frontend/Current app', args: { section: 'vehicle', data: 'frozen' },
  argTypes: { section: { control: 'select', options: ['fleet', 'vehicle'] }, data: { control: 'select', options: ['frozen', 'animated', 'stale', 'empty'] } },
  render: (args) => <Frontend key={`${args.section}-${args.data}`} {...args} />,
  parameters: { layout: 'fullscreen', docs: { description: { component: 'The actual FlightDisplay composition: fleet overview, vehicle sidebar, instruments, real map, mission download and pane controls. Frozen data supports precise styling; Animated runs the same mock stream as ?source=mock. Public basemap tiles require internet access.' } } },
} satisfies Meta<FrontendArgs>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Vehicle: Story = { globals: { viewport: { value: 'wide' } } };
export const Fleet: Story = { args: { section: 'fleet' }, globals: { viewport: { value: 'wide' } } };
export const Narrow: Story = { globals: { viewport: { value: 'narrow' } } };
export const Animated: Story = { args: { data: 'animated' } };
export const Stale: Story = { args: { data: 'stale' } };
export const EmptyFleet: Story = { args: { data: 'empty', section: 'fleet' } };
export const NoVehicle: Story = { args: { data: 'empty' } };
