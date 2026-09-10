import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { VehicleSelector } from '@/ui/VehicleSelector';
import { fixtureFleet, vehicle } from './fixtures';

function Selector({ count, lost }: { count: number; lost: boolean }) {
  const [selected, setSelected] = useState('1:1');
  const views = Array.from({ length: count }, (_, index) => ({ ...vehicle,
    key: `${String(index + 1)}:1`, sysId: index + 1,
    lifecycle: lost && index === count - 1 ? FleetEventType.VEHICLE_LOST : vehicle.lifecycle,
  }));
  return <VehicleSelector fleet={{ ...fixtureFleet, selected,
    order: views.map((view) => view.key), vehicles: Object.fromEntries(views.map((view) => [view.key, view])) }} onSelect={setSelected} />;
}
const meta = { title: 'Components/Vehicle selector', component: Selector, args: { count: 3, lost: false },
  argTypes: { count: { control: { type: 'range', min: 1, max: 12, step: 1 } } },
} satisfies Meta<typeof Selector>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Fleet: Story = {};
export const LostVehicle: Story = { args: { lost: true } };
export const SingleVehicleHidden: Story = { args: { count: 1 } };
