import type { ReactNode } from 'react';

import { Lever, LeverRow } from '@/ui/primitives';

/**
 * Shared camera controls; flight-specific actions stay with the vehicle map.
 *
 * `children` renders into the same row as the camera levers rather than a
 * row of its own — `MapPanel`'s Fit/Follow used to stack as a second boxed
 * cluster under this one, which read as two unrelated toolbars rather than
 * one. One row, one edge, one set of gaps.
 *
 * The row is right-justified and reads right to left as the operator's eye
 * lands on it: the camera-orientation pair (3D, Reset) sits at the far edge,
 * then zoom, then whatever flight-specific levers `children` supplies (Fit,
 * Follow) closest to the map content they act on.
 */
export function MapControls({ onZoom, onReset, onTilt, tilted, children }: {
  onZoom: (delta: number) => void;
  onReset: () => void;
  onTilt?: () => void;
  tilted?: boolean;
  children?: ReactNode;
}) {
  return (
    <LeverRow label="Map controls">
      {/* Terse on purpose. These two sit ON the map rather than beside it, so
          every character of label is a character of ground the operator came
          here to look at; the full sentence is on the pointer. The 28px
          target itself is not negotiable — field operation on a laptop is a
          standing requirement, and a control shrunk below a gloved fingertip
          stops being a control. */}
      {children}
      <Lever compact aria-label="Zoom in" title="Zoom in" onClick={() => { onZoom(1); }}>+</Lever>
      <Lever compact aria-label="Zoom out" title="Zoom out" onClick={() => { onZoom(-1); }}>−</Lever>
      <Lever title="Point north and remove tilt" onClick={onReset}>Reset</Lever>
      {onTilt && <Lever compact pressed={tilted} title="Tilt the camera; elevation terrain and 3D buildings are not configured" onClick={onTilt}>3D</Lever>}
    </LeverRow>
  );
}
