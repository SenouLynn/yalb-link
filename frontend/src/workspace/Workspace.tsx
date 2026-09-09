import { type ReactNode } from 'react';

/** Only composition knows the inventory; feature modules never register here. */
export const PANES = [
  { id: 'instruments', label: 'Instruments' },
  { id: 'map', label: 'Map' },
  { id: 'mission', label: 'Mission' },
] as const;
export type PaneId = typeof PANES[number]['id'];
export type PaneVisibility = Record<PaneId, boolean>;
export const DEFAULT_VISIBILITY: PaneVisibility = { instruments: true, map: true, mission: true };

export function PaneControls({ visible, onToggle }: {
  visible: PaneVisibility;
  onToggle: (id: PaneId) => void;
}) {
  return <div className="workspace__visibility" role="group" aria-label="Visible panels">
    <span className="label">Panels</span>
    {PANES.map(({ id, label }) => <button key={id} type="button" className="selector__button"
      aria-controls={`pane-${id}`} aria-pressed={visible[id]} onClick={() => { onToggle(id); }}>{label}</button>)}
  </div>;
}

export function WorkspaceShell({ topbar, children }: { topbar: ReactNode; children: ReactNode }) {
  return <div className="workspace"><header className="workspace__topbar">
    <h1>YALB <span className="label">Vehicle workspace</span></h1>{topbar}
  </header>{children}</div>;
}

export function WorkspacePanes({ visible, panes }: {
  visible: PaneVisibility;
  panes: Record<PaneId, ReactNode>;
}) {
  const shown = PANES.filter(({ id }) => visible[id]);
  return <main className={`workspace__main ${shown.map(({ id }) => `has-${id}`).join(' ')}`} aria-label="Vehicle panels">
    {PANES.map(({ id, label }) => <section key={id} id={`pane-${id}`} className={`pane pane--${id}`}
      hidden={!visible[id]} aria-label={label}>
      <h2 className="pane__title">{label}</h2><div className="pane__body">{panes[id]}</div>
    </section>)}
    {shown.length === 0 ? <p className="workspace__prompt">All panels hidden. Use the panel controls above to show a panel.</p> : null}
  </main>;
}
