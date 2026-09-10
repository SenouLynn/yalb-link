import type { Meta, StoryObj } from '@storybook/react-vite';
import { ArmControl } from '@/ui/ArmControl';
import { NOW, vehicle } from './fixtures';

const meta = { title: 'Components/Arm control', component: ArmControl,
  args: { view: vehicle, connected: true, source: 'mock', nowMs: NOW },
  // Keep stories on the production mock posture; these controls do not issue commands.
  argTypes: { source: { control: false }, view: { control: false } },
} satisfies Meta<typeof ArmControl>;
export default meta;
export const MockDisabled: StoryObj<typeof meta> = {};
