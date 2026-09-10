/** Composes the existing flight features in a persistent vehicle workspace. */

import type { FleetState, VehicleKey, VehicleView } from '@/fleet/state';
import type { TelemetrySample } from '@/logic/sample';
import {
  projectTrajectoryToGeo,
  resolvePredictiveTrajectory,
  type GeoCoordinate,
} from '@/logic/trajectory';
import { FleetOverview } from '@/fleet/FleetOverview';
import type { SavedCamera } from '@/map/FleetMap';
import { MapPanel } from '@/map/MapPanel';
import { missionLoaderFor } from '@/mission/source';
import { MissionPanel } from '@/mission/MissionPanel';
import { activeMissionSequence, missionGeometry } from '@/mission/model';
import {
  emptyMissionState,
  missionPanelState,
  visibleMissionState,
  type MissionViewState,
} from '@/mission/state';
import type { ReplayEventSource } from '@/stream/replay';
import type { StreamSource } from '@/stream/select';

import { ArmControl } from './ArmControl';
import { FamiliesPanel, SamplePanel } from './DevPanels';
import { GuidancePanel, HomePanel, RadioLinkPanel } from './InspectionPanel';
import { NO_VALUE } from './format';
import { Row } from './primitives';
import { hasDisplayValue, readFlight } from './readings';
import { ReplayControls } from './ReplayControls';
import { LinkRows, StateRows } from './StatusBar';
import { VehicleSelector } from './VehicleSelector';
import { useEffect, useMemo, useRef, useState } from 'react';
import { InstrumentPanel } from './InstrumentPanel';
import {
  DEFAULT_VISIBILITY,
  ViewsMenu,
  WorkspaceShell,
  WorkspaceSlots,
  type PanelContent,
  type PanelVisibility,
} from '@/workspace/Workspace';

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
  const controls = source === 'replay' ? <ReplayControls source={replay} /> : null;

  const [visible, setVisible] = useState<PanelVisibility>(DEFAULT_VISIBILITY);
  const toggle = (id: string) => {
    // Moving focus out before the panel unmounts; otherwise focus lands on the
    // body and the next Tab starts over at the top of the page.
    const panel = document.getElementById(`panel-${id}`);
    if (visible[id] === true && panel?.contains(document.activeElement) === true) {
      document.querySelector<HTMLInputElement>(`[aria-controls="panel-${id}"]`)?.focus();
    }
    setVisible((previous) => ({ ...previous, [id]: previous[id] !== true }));
  };

  return (
    <WorkspaceShell
      title="Ground control"
      meta={<span>{sourceLabel(source, fleet.connected)}</span>}
      viewBar={
        <>
          <button
            ref={navigation}
            type="button"
            className="lever"
            onClick={() => {
              if (section === 'fleet' && view) openVehicle(view.key);
              else setSection('fleet');
            }}
            disabled={section === 'fleet' && !view}
          >
            {section === 'fleet' ? 'Open selected vehicle' : '← Fleet'}
          </button>
          {section === 'vehicle' && (
            <>
              <VehicleSelector fleet={fleet} onSelect={onSelect} />
              <ViewsMenu visible={visible} onToggle={toggle} />
            </>
          )}
          {controls}
        </>
      }
    >
      {section === 'fleet' && (
        <FleetOverview
          fleet={fleet}
          nowMs={nowMs}
          source={source}
          onOpen={openVehicle}
          camera={fleetCamera}
        />
      )}
      <div className="vehicle-workspace" hidden={section !== 'vehicle'}>
        {view === undefined ? (
          <main className="shell__body">
            <p className="slot__empty">
              No vehicle selected.
              <br />
              {emptyHint(source, fleet.connected)}
            </p>
          </main>
        ) : (
          <SelectedFlightDisplay
            fleet={fleet}
            view={view}
            nowMs={nowMs}
            source={source}
            visible={visible}
            active={section === 'vehicle'}
          />
        )}
      </div>
    </WorkspaceShell>
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
    position: (
      <>
        <Row
          label="Latitude"
          value={position === null ? NO_VALUE : position.latDeg.toFixed(6)}
          tone={position === null ? 'dead' : 'normal'}
        />
        <Row
          label="Longitude"
          value={position === null ? NO_VALUE : position.lonDeg.toFixed(6)}
          tone={position === null ? 'dead' : 'normal'}
        />
        <Row label="Track" value={`${String(view.track.length)} / 500`} unit="pts" />
      </>
    ),
    mission: (
      <MissionPanel
        status={mission.status}
        snapshot={mission.snapshot}
        error={mission.error}
        geometry={geometry}
        activeSeq={activeSeq}
        onDownload={mission.onDownload}
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
    home: <HomePanel readings={readings} />,
    radiolink: <RadioLinkPanel readings={readings} />,
    instruments: <InstrumentPanel readings={readings} />,
    families: <FamiliesPanel view={view} nowMs={nowMs} />,
    sample: <SamplePanel view={view} />,
  };

  return <WorkspaceSlots visible={visible} content={content} />;
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
    void load(sysId, compId, controller.signal)
      .then((snapshot) => {
        if (request.current === controller)
          setState({ key, status: 'complete', snapshot, error: null });
      })
      .catch((error: unknown) => {
        if (request.current !== controller || controller.signal.aborted) return;
        setState({
          key,
          status: 'error',
          snapshot: null,
          error: error instanceof Error ? error.message : 'Mission download failed',
        });
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

/** What the display is showing, for the app bar. Never inferred from the data. */
function sourceLabel(source: StreamSource, connected: boolean): string {
  switch (source) {
    case 'mock':
      return 'MOCK';
    case 'replay':
      return 'REPLAY';
    default:
      return connected ? 'LIVE' : 'DISCONNECTED';
  }
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
