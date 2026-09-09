/** Composes the existing flight features in a persistent vehicle workspace. */

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

import { ArmControl } from './ArmControl';
import { hasDisplayValue, readFlight } from './readings';
import { ReplayControls } from './ReplayControls';
import { StatusBar } from './StatusBar';
import { VehicleSelector } from './VehicleSelector';
import { useEffect, useMemo, useRef, useState } from 'react';
import { InstrumentPanel } from './InstrumentPanel';
import { WorkspaceShell, WorkspacePanes, PaneControls, DEFAULT_VISIBILITY, type PaneVisibility } from '@/workspace/Workspace';

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

  const [visible, setVisible] = useState(DEFAULT_VISIBILITY);
  const toggle = (id: keyof PaneVisibility) => {
    const pane = document.getElementById(`pane-${id}`);
    if (visible[id] && pane?.contains(document.activeElement)) {
      document.querySelector<HTMLButtonElement>(`[aria-controls="pane-${id}"]`)?.focus();
    }
    setVisible((previous) => ({ ...previous, [id]: !previous[id] }));
  };
  const sourceLabel = source === 'live' ? (fleet.connected ? 'LIVE' : 'DISCONNECTED') : source.toUpperCase();
  return <WorkspaceShell topbar={<>
    <span className={`chip chip--${sourceLabel === 'LIVE' ? 'active' : 'caution'}`}>{sourceLabel}</span>
    <VehicleSelector fleet={fleet} onSelect={onSelect} />
    <PaneControls visible={visible} onToggle={toggle} />
    {controls}
  </>}>
    {view === undefined ? <>
      <aside className="workspace__sidebar" aria-label="Vehicle context"><div className="label">No vehicle selected</div></aside>
      <main className="workspace__main"><div className="panel empty"><div className="label">No vehicle</div>
        <p className="empty__hint">{emptyHint(source, fleet.connected)}</p></div></main>
    </> : <SelectedFlightDisplay fleet={fleet} view={view} nowMs={nowMs} source={source} visible={visible} />}
  </WorkspaceShell>;
}

/** Owns state that exists only while a vehicle is selected. */
function SelectedFlightDisplay({
  fleet,
  view,
  nowMs,
  source,
  visible,
}: {
  fleet: FleetState;
  view: VehicleView;
  nowMs: number;
  source: StreamSource;
  visible: PaneVisibility;
}) {
  const readings = readFlight(view, nowMs);
  const mission = useMission(view.key, view.sysId, view.compId, source);
  const geometry = useMemo(() => missionGeometry(mission.snapshot), [mission.snapshot]);
  const activeSeq = activeMissionSequence(view, nowMs);
  return <>
    <aside className="workspace__sidebar" aria-label="Vehicle context">
      <StatusBar view={view} nowMs={nowMs} connected={fleet.connected} source={source} />
      <ArmControl key={view.key} view={view} connected={fleet.connected} source={source} latest={fleet.commands[view.key]} staleLatest={fleet.commandsStale[view.key] === true} nowMs={nowMs} />
      <div className="workspace__position"><span className="label">Position · track</span>
        <p>{hasDisplayValue(readings.position)
          ? `${readings.position.value.latDeg.toFixed(6)}, ${readings.position.value.lonDeg.toFixed(6)}`
          : 'Position unavailable'}</p><p>{view.track.length} / 500 track points</p>
      </div>
    </aside>
    <WorkspacePanes visible={visible} panes={{
      instruments: <InstrumentPanel readings={readings} />,
      map:
      <MapPanel
        position={hasDisplayValue(readings.position) ? readings.position.value : null}
        track={view.track}
        trajectory={displayTrajectory(readings, view.sample)}
        mission={geometry}
        missionRevision={mission.snapshot}
        vehicleKey={view.key}
      />,
      mission: <MissionPanel
        status={mission.status}
        snapshot={mission.snapshot}
        error={mission.error}
        geometry={geometry}
        activeSeq={activeSeq}
        onDownload={mission.onDownload}
      />,
    }} />
  </>;
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
