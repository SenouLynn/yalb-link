/**
 * The fixed command bar above the map/video viewport.
 *
 * Arm/disarm used to be one more rail group — a togglable, and about to be
 * collapsible, `Group`. That is the wrong reach for a lever the product's own
 * principles call exceptional and within-reach at once: "commanding is
 * exceptional" and "data and levers within reach" are both true, and a closed
 * accordion satisfies neither. This bar is deliberately outside the panel
 * registry — it carries no visibility toggle, is never hidden by Views, and
 * is never inside an accordion. No menu diving, no optionality.
 *
 * Return-to-Home, Land, and Failsafe belong here too once they exist. They
 * are not built yet: `internal/command` and `frontend/src/commands` only
 * implement arm/disarm today, and mode switching needs an airframe-aware
 * mode map `StateRows` deliberately doesn't have. Their button hierarchy and
 * wiring are a follow-up — see the plan this shipped from — and should reuse
 * `ArmControl`'s confirm-then-attest safety pattern rather than inventing a
 * new one.
 */

import { ArmControl, type ArmControlProps } from './ArmControl';

export function CriticalCommandBar(props: ArmControlProps) {
  return (
    <div className="critical-bar">
      <ArmControl {...props} />
    </div>
  );
}
