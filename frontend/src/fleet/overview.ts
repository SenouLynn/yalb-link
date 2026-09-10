import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { FleetState, VehicleKey } from './state';
import { readFlight, hasDisplayValue } from '@/ui/readings';
import type { PositionResult } from '@/logic/position';

export interface FleetPosition extends PositionResult { key: VehicleKey }
/** Reuse the instruments' family clocks; never project missing data to 0,0. */
export function fleetPositions(fleet: FleetState, nowMs: number): FleetPosition[] {
  return fleet.order.flatMap(key => {
    const view = fleet.vehicles[key];
    if (!view || view.lifecycle === FleetEventType.VEHICLE_LOST) return [];
    const { position } = readFlight(view, nowMs);
    if (!hasDisplayValue(position)) return [];
    const p = position.value;
    return Number.isFinite(p.latDeg) && Math.abs(p.latDeg) <= 90
      && Number.isFinite(p.lonDeg) && Math.abs(p.lonDeg) <= 180 ? [{ ...p, key }] : [];
  });
}
