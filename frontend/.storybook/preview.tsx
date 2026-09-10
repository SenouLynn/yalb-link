import type { Preview } from '@storybook/react-vite';
import '../src/ui/system.css';
import '../src/ui/display.css';
import './preview.css';

const preview: Preview = {
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    controls: { expanded: true },
    options: { storySort: { order: ['Components', 'Panels', 'Composition', 'Frontend'] } },
    backgrounds: { options: { cockpit: { name: 'Cockpit', value: '#0d1013' } } },
    viewport: { options: {
      narrow: { name: 'Narrow · 390px', styles: { width: '390px', height: '844px' } },
      wide: { name: 'Desktop · 1440px', styles: { width: '1440px', height: '900px' } },
    } },
  },
  initialGlobals: { backgrounds: { value: 'cockpit' } },
};
export default preview;
