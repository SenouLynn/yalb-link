/** The assembled mini flight display for one selected vehicle. */

import type { FleetState, VehicleKey, VehicleView } from '@/fleet/state';
import type { TelemetrySample } from '@/logic/sample';
import {
  projectTrajectoryToGeo,
  resolvePredictiveTrajectory,
  type GeoCoordinate,
} from '@/logic/trajectory';
import { MapPanel } from '@/map/MapPanel';
import { missionLoaderFor } from '@/mission/source';
import { MissionPanel } from '@/mission/MissionPanel';
import { activeMissionSequence, missionGeometry } from '@/mission/model';
import { emptyMissionState, missionPanelState, visibleMissionState, type MissionViewState } from '@/mission/state';
import type { ReplayEventSource } from '@/stream/replay';
import type { StreamSource } from '@/stream/select';

import { AttitudeIndicator } from './AttitudeIndicator';
import { ArmControl } from './ArmControl';
import { HeadingIndicator } from './HeadingIndicator';
import { NO_VALUE, num, signed } from './format';
import { hasDisplayValue, readFlight } from './readings';
import { Readout } from './Readout';
import { ReplayControls } from './ReplayControls';
import { StatusBar } from './StatusBar';
import { VehicleSelector } from './VehicleSelector';
import { useEffect, useRef, useState } from 'react';

export interface FlightDisplayProps {
  fleet: FleetState;
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
}: FlightDisplayProps) {
  const view = fleet.selected === null ? undefined : fleet.vehicles[fleet.selected];
  const controls = source === 'replay' ? <ReplayControls source={replay} /> : null;

  if (view === undefined) {
    return <EmptyFleet connected={fleet.connected} source={source} controls={controls} />;
  }

  return <SelectedFlightDisplay
    fleet={fleet}
    view={view}
    nowMs={nowMs}
    source={source}
    controls={controls}
    onSelect={onSelect}
  />;
}

/** Owns state that exists only while a vehicle is selected. */
function SelectedFlightDisplay({
  fleet,
  view,
  nowMs,
  source,
  controls,
  onSelect,
}: {
  fleet: FleetState;
  view: VehicleView;
  nowMs: number;
  source: StreamSource;
  controls: React.ReactNode;
  onSelect: (key: VehicleKey) => void;
}) {
  const readings = readFlight(view, nowMs);
  const mission = useMission(view.key, view.sysId, view.compId, source);
  const geometry = missionGeometry(mission.snapshot);
  const activeSeq = activeMissionSequence(view, nowMs);
  const { position, flightPath, battery } = readings;

  const altitude = hasDisplayValue(position) ? position.value : null;
  const path = hasDisplayValue(flightPath) ? flightPath.value : null;
  const power = hasDisplayValue(battery) ? battery.value : null;

  return (
    <div className="display">
      <StatusBar view={view} nowMs={nowMs} connected={fleet.connected} source={source} />

      <ArmControl view={view} connected={fleet.connected} source={source} latest={fleet.commands[view.key]} nowMs={nowMs} />

      {controls}

      <VehicleSelector fleet={fleet} onSelect={onSelect} />

      <div className="instruments">
        <AttitudeIndicator reading={readings.attitude} />

        <div className="readouts">
          <Readout
            label="Altitude"
            note={altitude === null ? 'datum unknown' : datumNote(altitude.altRef)}
            value={altitude === null ? NO_VALUE : num(altitude.altM)}
            unit="m"
            reading={position}
            wide
          />

          <Readout
            label="Ground speed"
            value={path === null ? NO_VALUE : num(path.groundSpeedMps)}
            unit="m/s"
            reading={flightPath}
          />

          <Readout
            label="Climb"
            value={path === null ? NO_VALUE : signed(path.climbMps)}
            unit="m/s"
            reading={flightPath}
          />

          <Readout
            label="Battery"
            value={power === null ? NO_VALUE : num(power.voltageV, 2)}
            unit="V"
            reading={battery}
          />

          <Readout
            label="Remaining"
            value={power === null ? NO_VALUE : num(power.remainingPct, 0)}
            unit="%"
            reading={battery}
          />
        </div>
      </div>

      <HeadingIndicator reading={readings.heading} />

      <MapPanel
        position={hasDisplayValue(readings.position) ? readings.position.value : null}
        track={view.track}
        trajectory={displayTrajectory(readings, view.sample)}
        mission={geometry}
      />

      {/* Passed field by field: MissionViewState carries a `key`, and spreading
          it hands React a reconciliation key instead of a prop. */}
      <MissionPanel
        status={mission.status}
        snapshot={mission.snapshot}
        error={mission.error}
        geometry={geometry}
        activeSeq={activeSeq}
        onDownload={mission.onDownload}
      />
    </div>
  );
}

function useMission(key: VehicleKey, sysId: number, compId: number, source: StreamSource) {
  const [state, setState] = useState<MissionViewState>(() => emptyMissionState(key));
  const request = useRef<AbortController | null>(null);

  useEffect(() => {
    request.current?.abort();
    setState(emptyMissionState(key));
    return () => request.current?.abort();
  }, [key]);

  const visible = visibleMissionState(state, key);
  const load = missionLoaderFor(source);
  const onDownload = () => {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setState({ key, status: 'loading', snapshot: null, error: null });
    void load(sysId, compId, controller.signal).then((snapshot) => {
      if (request.current === controller) setState({ key, status: 'complete', snapshot, error: null });
    }).catch((error: unknown) => {
      if (request.current !== controller || controller.signal.aborted) return;
      setState({ key, status: 'error', snapshot: null, error: error instanceof Error ? error.message : 'Mission download failed' });
    });
  };
  return { ...missionPanelState(visible), onDownload };
}

/**
 * Builds a read-only prediction exclusively from display-approved live values.
 * Ground speed is deliberate: no vehicle-specific stall speed is configured.
 * Stale attitude drops curvature but cannot keep an old turn on screen.
 */
function displayTrajectory(
  readings: ReturnType<typeof readFlight>,
  sample: TelemetrySample,
): GeoCoordinate[] {
  if (
    !hasDisplayValue(readings.position) ||
    !hasDisplayValue(readings.heading) ||
    !hasDisplayValue(readings.flightPath)
  ) {
    return [];
  }

  const liveAttitude = hasDisplayValue(readings.attitude);
  const trajectorySample: TelemetrySample = {
    sourceMessage: 'DISPLAY_TRAJECTORY',
    receivedAtMs: sample.receivedAtMs,
    headingDeg: readings.heading.value.headingDeg,
    groundspeedMps: readings.flightPath.value.groundSpeedMps,
    ...(liveAttitude
      ? {
          rollRad: sample.rollRad,
          pitchRad: sample.pitchRad,
          pitchspeedRadS: sample.pitchspeedRadS,
          yawspeedRadS: sample.yawspeedRadS,
        }
      : {}),
  };

  const offsets = resolvePredictiveTrajectory(trajectorySample);
  const last = offsets[offsets.length - 1];

  // A stationary vehicle has no visible future path.
  if (last === undefined || Math.hypot(last.northM, last.eastM) < 0.01) {
    return [];
  }

  return projectTrajectoryToGeo(readings.position.value, offsets);
}

/** Names the altitude datum explicitly; the two differ by field elevation. */
function datumNote(ref: 'RELATIVE' | 'MSL'): string {
  return ref === 'RELATIVE' ? 'above home' : 'above sea level';
}

function EmptyFleet({
  connected,
  source,
  controls,
}: {
  connected: boolean;
  source: StreamSource;
  controls: React.ReactNode;
}) {
  return (
    <div className="display">
      {controls}

      <div className="panel empty">
        <div className="label">No vehicle</div>
        <p className="empty__hint">{emptyHint(source, connected)}</p>
      </div>
    </div>
  );
}

function emptyHint(source: StreamSource, connected: boolean): string {
  switch (source) {
    case 'mock':
      return 'Replaying fixtures. The mock vehicle appears on the first frame.';
    case 'replay':
      return 'Replaying a recording. Pick one, then press play.';
    default:
      return connected
        ? 'Connected to the backend. Waiting for a heartbeat.'
        : 'Not connected to the backend. Start it, or open ?source=mock.';
  }
}
