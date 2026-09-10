import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { ViewsMenu, DEFAULT_VISIBILITY, type PanelVisibility } from '@/workspace/Workspace';

function Views() {
  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);
  return (
    <div className="shell__bar shell__bar--view" style={{ width: 420, maxWidth: '100%' }}>
      <ViewsMenu
        visible={visible}
        onToggle={(id) => {
          setVisible((value) => ({ ...value, [id]: value[id] !== true }));
        }}
      />
    </div>
  );
}

const meta = { title: 'Shell/Views menu', component: Views } satisfies Meta<typeof Views>;
export default meta;
export const Interactive: StoryObj<typeof meta> = {};
