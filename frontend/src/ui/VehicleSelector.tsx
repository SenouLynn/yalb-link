/** Vehicle picker. Rendered only when there is a choice to make. */

import { LeverRow, Lever } from '@/ui/primitives';

import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { FleetState, VehicleKey } from '@/fleet/state';

export interface VehicleSelectorProps {
  fleet: FleetState;
  onSelect: (key: VehicleKey) => void;
}

/**
 * Lists the fleet in identity order.
 *
 * Returns nothing for a single vehicle: a picker with one option is a control
 * that cannot do anything, and on a display this small the row it occupies is
 * better spent on an instrument.
 */
export function VehicleSelector({ fleet, onSelect }: VehicleSelectorProps) {
  if (fleet.order.length < 2) {
    return null;
  }

  /*
   * A lever row, not a `Group`. This renders into the view bar, and a section
   * header bar sitting inside a toolbar reads as a panel that lost its panel —
   * it also stacks the bar three rows deep and puts "FLEET" on a different
   * baseline from the two levers either side of it. The count is dropped rather
   * than relocated: the levers themselves are the count, and one per vehicle is
   * already visible.
   */
  return (
    <LeverRow label="Select vehicle">
      {fleet.order.map((key) => {
          const view = fleet.vehicles[key];

          if (view === undefined) {
            return null;
          }

          const lost = view.lifecycle === FleetEventType.VEHICLE_LOST;

          return (
            <Lever
              key={key}
              pressed={fleet.selected === key}
              onClick={() => {
                onSelect(key);
              }}
            >
              {view.sysId}:{view.compId}
              {lost ? ' · LOST' : ''}
            </Lever>
          );
      })}
    </LeverRow>
  );
}
