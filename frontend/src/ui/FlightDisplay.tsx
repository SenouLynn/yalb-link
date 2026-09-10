/** Routes between the two views and owns the app bar they share. */

import { useEffect, useRef, useState } from 'react';

import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { FleetState, VehicleKey, VehicleView } from '@/fleet/state';
import { FleetOverview } from '@/fleet/FleetOverview';
import type { SavedCamera } from '@/map/FleetMap';
import type { ReplayEventSource } from '@/stream/replay';
import type { StreamSource } from '@/stream/select';
import { VehiclePane } from '@/vehicle/VehiclePane';
import { ViewBar, WorkspaceShell } from '@/workspace/Workspace';

import { sourceLabel } from './format';
import { Glance, Lever } from './primitives';
import { ReplayControls } from './ReplayControls';

export interface FlightDisplayProps {
  fleet: FleetState;
  initialSection?: 'fleet' | 'vehicle';
  /** Injected clock; freshness is measured against it. */
  nowMs: number;
  /** Where the data comes from. Drives what the display claims it is showing. */
  source: StreamSource;
  /** The running replay, when this page is one. */
  replay?: ReplayEventSource | null;
  onSelect: (key: VehicleKey) => void;
}

export function FlightDisplay({
  fleet,
  nowMs,
  source,
  replay = null,
  onSelect,
  initialSection,
}: FlightDisplayProps) {
  const [section, setSection] = useState<'fleet' | 'vehicle'>(
    initialSection ?? (source === 'replay' ? 'vehicle' : 'fleet'),
  );
  const fleetCamera = useRef<SavedCamera | null>(null);
  const navigation = useRef<HTMLButtonElement>(null);
  const openVehicle = (key: VehicleKey) => {
    onSelect(key);
    setSection('vehicle');
  };
  useEffect(() => {
    navigation.current?.focus();
  }, [section]);
  const view = fleet.selected === null ? undefined : fleet.vehicles[fleet.selected];

  /*
   * The replay transport is source chrome, not view chrome: it drives the clock
   * both views are read against, so it stays with whichever one is on screen.
   */
  const controls = source === 'replay' ? <ReplayControls source={replay} /> : null;

  return (
    <WorkspaceShell
      meta={<span>{sourceLabel(source, fleet.connected)}</span>}
      nav={
        <>
          {/*
            * Where the operator is, and — once they are on a vehicle — which one.
            * Both were a row lower until the app bar was carrying nothing but the
            * product's own name, which is a fact that never changes.
            */}
          <Lever
            ref={navigation}
            onClick={() => {
              if (section === 'fleet' && view) openVehicle(view.key);
              else setSection('fleet');
            }}
            disabled={section === 'fleet' && !view}
          >
            {section === 'fleet' ? 'Open selected vehicle' : '← Fleet'}
          </Lever>

          {/*
            * Identity, not a picker.
            *
            * Switching aircraft goes back through the fleet: a control that
            * changes which vehicle the whole screen is about does not belong on
            * a toolbar beside the panel toggles, one stray click away from
            * moving the operator onto a different airframe while they are
            * reading the one in front of them. Coming out to the fleet makes
            * that a deliberate act, and the roster is where the comparison that
            * informs it lives. This states which one is on screen and offers no
            * way to change it.
            */}
          {section === 'vehicle' && view !== undefined && (
            <Glance
              label="Vehicle"
              value={lost(view) ? `${view.key} · LOST` : view.key}
              tone={lost(view) ? 'caution' : 'normal'}
              title="Return to the fleet to select a different vehicle"
            />
          )}
        </>
      }
    >
      {section === 'fleet' && (
        <div className="view">
          {controls === null ? null : <ViewBar>{controls}</ViewBar>}
          <FleetOverview
            fleet={fleet}
            nowMs={nowMs}
            source={source}
            onOpen={openVehicle}
            camera={fleetCamera}
          />
        </div>
      )}

      <VehiclePane
        fleet={fleet}
        view={view}
        nowMs={nowMs}
        source={source}
        active={section === 'vehicle'}
        controls={section === 'vehicle' ? controls : null}
      />
    </WorkspaceShell>
  );
}

/** Whether the vehicle has stopped being heard, which its identity has to say. */
function lost(view: VehicleView): boolean {
  return view.lifecycle === FleetEventType.VEHICLE_LOST;
}
