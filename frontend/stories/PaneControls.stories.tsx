import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { PaneControls, DEFAULT_VISIBILITY } from '@/workspace/Workspace';

function Controls() {
  const [visible, setVisible] = useState(DEFAULT_VISIBILITY);
  return <PaneControls visible={visible} onToggle={(id) => { setVisible((value) => ({ ...value, [id]: !value[id] })); }} />;
}
const meta = { title: 'Components/Pane controls', component: Controls } satisfies Meta<typeof Controls>;
export default meta;
export const Interactive: StoryObj<typeof meta> = {};
