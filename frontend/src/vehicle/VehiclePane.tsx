/**
 * The vehicle workspace: one aircraft, read whole.
 *
 * Split out of `FlightDisplay` so the pane is a thing that can be mounted, and
 * so that panel visibility is owned where the panels are. The Views control is
 * a property of this pane — an application bar that outlives the pane has no
 * business holding it — which is also why the pane renders its own view bar
 * rather than posting chrome up into the shell's.
 *
 * The pane stays mounted while the operator is on the fleet view. Unmounting
 * would tear down the map context and drop an in-flight mission download, and
 * it would reset panel visibility every time they looked at the fleet.
 */

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';

import type { FleetState, VehicleKey, VehicleView } from '@/fleet/state';
import type { TelemetrySample } from '@/logic/sample';
import {
  projectTrajectoryToGeo,
  resolvePredictiveTrajectory,
  type GeoCoordinate,
} from '@/logic/trajectory';
import { MapPanel } from '@/map/MapPanel';
import { missionLoaderFor } from '@/mission/source';
import { MissionPanel, WaypointList } from '@/mission/MissionPanel';
import { activeMissionSequence, missionGeometry } from '@/mission/model';
import {
  emptyMissionState,
  missionPanelState,
  visibleMissionState,
  type MissionViewState,
} from '@/mission/state';
import type { StreamSource } from '@/stream/select';
import { ArmControl } from '@/ui/ArmControl';
import { FamiliesPanel, SamplePanel } from '@/ui/DevPanels';
import { latLon } from '@/ui/format';
import { GuidancePanel, HomeRows, RadioLinkPanel } from '@/ui/InspectionPanel';
import { InstrumentPanel } from '@/ui/InstrumentPanel';
import { Group, Row } from '@/ui/primitives';
import { hasDisplayValue, readFlight } from '@/ui/readings';
import { LinkRows, StateRows } from '@/ui/StatusBar';
import {
  DEFAULT_VISIBILITY,
  ViewBar,
  ViewsMenu,
  WorkspaceSlots,
  toggleSection,
  type PanelContent,
  type PanelVisibility,
  type SectionId,
} from '@/workspace/Workspace';

import { GlanceBar } from './GlanceBar';

export interface VehiclePaneProps {
  fleet: FleetState;
  /** The selected vehicle, or undefined when the fleet has not named one. */
  view: VehicleView | undefined;
  /** Injected clock; freshness is measured against it. */
  nowMs: number;
  source: StreamSource;
  /** Whether this pane is the view on screen. Hidden, never unmounted. */
  active: boolean;
  /** Source-level chrome that belongs under the app bar — the replay transport. */
  controls?: ReactNode;
}

export function VehiclePane({ fleet, view, nowMs, source, active, controls }: VehiclePaneProps) {
  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);

  const toggle = (id: string) => {
    // Moving focus out before the panel is hidden; otherwise focus lands on the
    // body and the next Tab starts over at the top of the page.
    const panel = document.getElementById(`panel-${id}`);
    if (visible[id] === true && panel?.contains(document.activeElement) === true) {
      document.querySelector<HTMLInputElement>(`[aria-controls="panel-${id}"]`)?.focus();
    }
    setVisible((previous) => ({ ...previous, [id]: previous[id] !== true }));
  };

  const toggleWholeSection = (id: SectionId) => {
    setVisible((previous) => toggleSection(id, previous));
  };

  return (
    <div className="view vehicle-workspace" hidden={!active}>
      <ViewBar
        trailing={
          <ViewsMenu visible={visible} onToggle={toggle} onToggleSection={toggleWholeSection} />
        }
      >
        {controls}
        {view === undefined ? null : (
          <GlanceBar view={view} readings={readFlight(view, nowMs)} nowMs={nowMs} />
        )}
      </ViewBar>

      {view === undefined ? (
        <main className="shell__body">
          <p className="slot__empty">No vehicle selected. {emptyHint(source, fleet.connected)}</p>
        </main>
      ) : (
        <SelectedFlightDisplay
          fleet={fleet}
          view={view}
          nowMs={nowMs}
          source={source}
          visible={visible}
          active={active}
        />
      )}
    </div>
  );
}

/** Owns state that exists only while a vehicle is selected. */
function SelectedFlightDisplay({
  fleet,
  view,
  nowMs,
  source,
  visible,
  active,
}: {
  fleet: FleetState;
  view: VehicleView;
  nowMs: number;
  source: StreamSource;
  visible: PanelVisibility;
  active: boolean;
}) {
  const readings = readFlight(view, nowMs);
  const mission = useMission(view.key, view.sysId, view.compId, source);
  const geometry = useMemo(() => missionGeometry(mission.snapshot), [mission.snapshot]);
  const activeSeq = activeMissionSequence(view, nowMs);
  const position = hasDisplayValue(readings.position) ? readings.position.value : null;
  const status = { view, nowMs, connected: fleet.connected, source };

  /*
   * One node per registered panel id. Composition is the only thing that knows
   * both the registry and the feature modules, which is what keeps a feature
   * from being able to put itself on screen.
   */
  const content: PanelContent = {
    link: <LinkRows {...status} />,
    state: <StateRows {...status} />,
    command: (
      <ArmControl
        active={active}
        key={view.key}
        view={view}
        connected={fleet.connected}
        source={source}
        latest={fleet.commands[view.key]}
        staleLatest={fleet.commandsStale[view.key] === true}
        nowMs={nowMs}
      />
    ),
    /*
     * Where it is, and what that is measured against.
     *
     * Home has no section of its own: it is the datum the position above it and
     * the altitude tape across the screen are both measured against, and a
     * heading between them read as an unrelated fact that happened to be
     * nearby. It comes from a different family with a different lifetime, so
     * `HomeRows` names its own source rather than borrowing this header's.
     */
    position: (
      <Group label="Position" reading={readings.position}>
        <Row
          label="Lat / lon"
          value={latLon(position?.latDeg, position?.lonDeg)}
          tone={position === null ? 'dead' : 'normal'}
        />
        <Row label="Track" value={`${String(view.track.length)} / 500`} unit="pts" />
        <HomeRows readings={readings} />
      </Group>
    ),
    mission: (
      <MissionPanel
        nowMs={nowMs}
        status={mission.status}
        snapshot={mission.snapshot}
        error={mission.error}
        activeSeq={activeSeq}
        onDownload={mission.onDownload}
      />
    ),
    waypoints: (
      <WaypointList
        nowMs={nowMs}
        snapshot={mission.snapshot}
        geometry={geometry}
        activeSeq={activeSeq}
      />
    ),
    map: active ? (
      <MapPanel
        position={position}
        track={view.track}
        trajectory={displayTrajectory(readings, view.sample)}
        mission={geometry}
        missionRevision={mission.snapshot}
        vehicleKey={view.key}
      />
    ) : null,
    guidance: <GuidancePanel readings={readings} />,
    radiolink: <RadioLinkPanel readings={readings} />,
    instruments: <InstrumentPanel readings={readings} />,
    families: <FamiliesPanel view={view} nowMs={nowMs} />,
    sample: <SamplePanel view={view} />,
  };

  return <WorkspaceSlots visible={visible} content={content} />;
}

function useMission(key: VehicleKey, sysId: number, compId: number, source: StreamSource) {
  const [state, setState] = useState<MissionViewState & { source: StreamSource }>(() => ({ ...emptyMissionState(key), source }));
  const request = useRef<AbortController | null>(null);

  const visible = state.source === source ? visibleMissionState(state, key) : emptyMissionState(key);
  const load = missionLoaderFor(source);
  const onDownload = useCallback(() => {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setState({ source, key, status: 'loading', snapshot: null, error: null });
    void load(sysId, compId, controller.signal)
      .then((snapshot) => {
        if (request.current === controller && !controller.signal.aborted)
          setState({ source, key, status: 'complete', snapshot, error: null });
      })
      .catch((error: unknown) => {
        if (request.current !== controller || controller.signal.aborted) return;
        setState({
          source,
          key,
          status: 'error',
          snapshot: null,
          error: error instanceof Error ? error.message : 'Mission download failed',
        });
      });
  }, [key, sysId, compId, load, source]);
  useEffect(() => {
    request.current?.abort();
    setState({ ...emptyMissionState(key), source });
    if (source === 'mock') onDownload();
    return () => request.current?.abort();
  }, [key, source, onDownload]);
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
