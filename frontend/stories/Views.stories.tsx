import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { ViewBar, ViewsMenu, DEFAULT_VISIBILITY, toggleSection, type PanelVisibility } from '@/workspace/Workspace';

/** In the bar it ships in, held to the right edge the way the vehicle pane holds it. */
function Views() {
  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);
  return (
    <div style={{ width: 420, maxWidth: '100%' }}>
      <ViewBar
        trailing={
          <ViewsMenu
            visible={visible}
            onToggle={(id) => {
              setVisible((value) => ({ ...value, [id]: value[id] !== true }));
            }}
            onToggleSection={(id) => {
              setVisible((value) => toggleSection(id, value));
            }}
          />
        }
      />
    </div>
  );
}

const meta = { title: 'Shell/Views menu', component: Views } satisfies Meta<typeof Views>;
export default meta;
export const Interactive: StoryObj<typeof meta> = {};
