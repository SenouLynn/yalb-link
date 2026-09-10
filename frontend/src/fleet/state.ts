/**
 * Fleet-aware view state.
 *
 * Keyed by the full `(sysid, compid)` identity rather than system ID alone: a
 * gimbal and its autopilot share a system ID and are not the same thing, and
 * merging their telemetry would put one vehicle's attitude on another's
 * instrument.
 */

import { FleetEventType, type FleetEvent } from '@/gen/gcs/v1/fleet_pb';
import type { TelemetryEvent } from '@/gen/gcs/v1/telemetry_pb';
import type { HeartbeatState } from '@/gen/gcs/v1/vehicle_pb';
import type { CommandTransaction } from '@/gen/gcs/v1/commands_pb';
import { isFresh } from '@/logic/freshness';
import { accumulateGeoTrack, type GeoPoint } from '@/logic/geoTrack';
import { lastLegM } from '@/logic/odometer';
import { sampleFromEvent, type TelemetrySample } from '@/logic/sample';

import type { StreamEvent } from '@/stream/events';

import { observedAtMs } from './observed';

/**
 * How long a telemetry family stays trustworthy.
 *
 * Five seconds is generous against the slowest requested rate (1 Hz) and short
 * enough that an operator notices a value has stopped updating before acting
 * on it. The boundary itself is `logic/freshness`'s: strictly less than the
 * TTL is fresh.
 */
export const TELEMETRY_TTL_MS = 5_000;

/** A vehicle's identity, rendered the way logs and route tables refer to it. */
export type VehicleKey = string;

export function vehicleKey(sysId: number, compId: number): VehicleKey {
  return `${String(sysId)}:${String(compId)}`;
}

/** The accumulated view of one vehicle. */
export interface VehicleView {
  key: VehicleKey;
  sysId: number;
  compId: number;

  /** The most recent fleet event type. Absent until one arrives. */
  lifecycle: FleetEventType | undefined;
  heartbeat: HeartbeatState | undefined;
  lastFleetAtMs: number | undefined;

  /** Every telemetry field seen so far, merged. Latest value per field wins. */
  sample: TelemetrySample;

  /** In-memory geodetic breadcrumb trail, appended only by position families. */
  track: GeoPoint[];

  /**
   * Great-circle distance flown, metres, accumulated as fixes arrive.
   *
   * Held here rather than measured off `track` because the track is a
   * fixed-capacity ring: a distance summed over what it still holds would mean
   * "the last hundred seconds" while reading as the distance flown, and would
   * shrink as the sortie went on. See `logic/odometer`.
   */
  odometerM: number;

  /**
   * When the first position fix landed, for time in the air.
   *
   * The first *fix*, not the first frame: a vehicle heartbeats on the bench
   * long before it has a position, and an elapsed clock started there reports a
   * sortie that has not begun.
   */
  firstFixAtMs: number | undefined;

  /**
   * When the backend observed each MAVLink family, by the resolver-facing
   * source name. Merged fields never expire on their own, so this is what
   * tells the display that a value it still holds has stopped being true.
   *
   * This is the backend's `observed_at`, not the moment the browser received
   * the event. The difference matters on reconnect: the hub replays retained
   * state, so a browser that stamped arrival time would show telemetry from
   * ten minutes ago as if it had just landed — the one thing this display
   * exists to make impossible. It assumes the backend and the browser agree
   * roughly on the wall clock, which holds for the localhost and Compose
   * topologies this milestone covers.
   */
  familySeenMs: Readonly<Record<string, number>>;

  /** Last time anything at all arrived for this vehicle. */
  lastSeenMs: number;
}

export interface FleetState {
  vehicles: Readonly<Record<VehicleKey, VehicleView>>;
  /** Ascending by (sysid, compid), so the UI order never depends on arrival. */
  order: readonly VehicleKey[];
  /** The vehicle being displayed, or null when the fleet is empty. */
  selected: VehicleKey | null;
  /**
   * Whether the operator picked the current vehicle.
   *
   * An automatic selection is provisional and gets revisited when the fleet
   * changes — otherwise the first vehicle to say anything would keep the
   * display forever, even once a lower-numbered one appeared. An explicit
   * choice is not revisited: the operator is looking at that aircraft, and
   * moving them off it because another vehicle arrived would be the display
   * overriding a decision they made.
   */
  selectionPinned: boolean;
  /** Whether the browser currently has the backend stream. */
  connected: boolean;
  /** Latest command event per vehicle. Not used to infer armed state. */
  commands: Readonly<Record<VehicleKey, CommandTransaction>>;
  /**
   * Vehicles whose retained transaction predates a gap in the stream.
   *
   * ADR 0004 deliberately keeps command events out of the hub's bootstrap: a
   * transaction is history, not current vehicle state. A reconnect therefore
   * replays fleet and telemetry but never the commands issued while the
   * browser was away, so what is still on screen may already have been
   * superseded by a transaction this browser never saw. The entry is marked
   * rather than dropped, because the operator did issue that command and a
   * blank would read as "nothing happened".
   */
  commandsStale: Readonly<Record<VehicleKey, true>>;
}

export const initialFleetState: FleetState = {
  vehicles: {},
  order: [],
  selected: null,
  selectionPinned: false,
  connected: false,
  commands: {},
  commandsStale: {},
};

/** A vehicle the operator can act on: seen, and not currently reported lost. */
export function isActive(view: VehicleView): boolean {
  return view.lifecycle !== FleetEventType.VEHICLE_LOST;
}

/** Reports whether a resolver's source family is still fresh. */
export function isFamilyFresh(
  view: VehicleView,
  family: string,
  nowMs: number,
  ttlMs: number = TELEMETRY_TTL_MS,
): boolean {
  const seen = view.familySeenMs[family];

  if (seen === undefined) {
    return false;
  }

  return isFresh(seen, nowMs, ttlMs);
}

export type FleetAction =
  | { type: 'stream'; event: StreamEvent }
  | { type: 'select'; key: VehicleKey };

export function fleetReducer(state: FleetState, action: FleetAction): FleetState {
  switch (action.type) {
    case 'select':
      // Selecting an unknown vehicle is ignored rather than blanking the
      // display: the request raced a vehicle disappearing, and the operator is
      // better served by keeping what they were looking at.
      return action.key in state.vehicles
        ? { ...state, selected: action.key, selectionPinned: true }
        : state;

    case 'stream':
      return applyStreamEvent(state, action.event);

    default:
      return state;
  }
}

/**
 * Marks every vehicle whose transaction is currently on screen.
 *
 * EventSource reports a bare `error` for each failed attempt, so this runs
 * repeatedly through one outage. It returns the existing map when there is
 * nothing new to mark, so retrying does not re-render the fleet each time.
 */
function staleKeys(state: FleetState): Readonly<Record<VehicleKey, true>> {
  const keys = Object.keys(state.commands);

  if (keys.every((key) => state.commandsStale[key] === true)) {
    return state.commandsStale;
  }

  return Object.fromEntries(keys.map((key) => [key, true]));
}

function applyStreamEvent(state: FleetState, event: StreamEvent): FleetState {
  switch (event.kind) {
    case 'connection':
      // Losing the stream is the moment command history stops arriving, so the
      // mark goes on here rather than on the reconnect: while the browser is
      // away the vehicle may answer a command it will never be told about.
      return event.connected
        ? { ...state, connected: true }
        : { ...state, connected: false, commandsStale: staleKeys(state) };

    case 'fleet':
      return withVehicle(state, applyFleet(event.event, event.receivedAtMs));

    case 'telemetry':
      return withVehicle(state, applyTelemetry(event.event, event.receivedAtMs));

    case 'command': {
      const id = event.event.vehicleId;
      if (id === undefined) return state;
      const key = vehicleKey(id.systemId, id.componentId);
      // A transaction that just arrived is current by definition, so it clears
      // the mark for its own vehicle and leaves every other vehicle's alone.
      const commandsStale = key in state.commandsStale
        ? Object.fromEntries(Object.entries(state.commandsStale).filter(([marked]) => marked !== key))
        : state.commandsStale;
      return { ...state, commands: { ...state.commands, [key]: event.event }, commandsStale };
    }

    case 'reset':
      // Everything accumulated is discarded, but an operator's explicit choice
      // of vehicle is theirs and survives: a replay rewinding to its start
      // should not also change which aircraft they were watching.
      return {
        ...initialFleetState,
        connected: state.connected,
        selected: state.selectionPinned ? state.selected : null,
        selectionPinned: state.selectionPinned,
      };

    default:
      return state;
  }
}

/** An update to one vehicle, expressed as a function of its previous view. */
interface VehicleUpdate {
  sysId: number;
  compId: number;
  apply(previous: VehicleView): VehicleView;
}

function applyFleet(event: FleetEvent, receivedAtMs: number): VehicleUpdate | null {
  const id = event.vehicleId;

  if (id === undefined) {
    return null;
  }

  return {
    sysId: id.systemId,
    compId: id.componentId,
    apply: (previous) => ({
      ...previous,
      lifecycle: event.type,
      // A LOST event carries the last known heartbeat; keeping the previous one
      // when a new event omits it avoids blanking the mode and armed state at
      // exactly the moment the operator wants to read them.
      heartbeat: event.heartbeat ?? previous.heartbeat,
      lastFleetAtMs: receivedAtMs,
      lastSeenMs: receivedAtMs,
      sample: mergeHeartbeat(previous.sample, event.heartbeat ?? previous.heartbeat),
    }),
  };
}

function applyTelemetry(event: TelemetryEvent, receivedAtMs: number): VehicleUpdate | null {
  const id = event.vehicleId;

  if (id === undefined) {
    return null;
  }

  const partial = sampleFromEvent(event, receivedAtMs);

  // Fall back to receipt only when the backend sent no stamp at all, which
  // means an older backend or a payload outside the telemetry contract.
  const observedMs = observedAtMs(event) ?? receivedAtMs;

  return {
    sysId: id.systemId,
    compId: id.componentId,
    apply: (previous) => {
      // Accumulate the event partial, not the merged vehicle sample. Otherwise
      // every attitude/battery frame would re-append the last known position.
      const track = accumulateGeoTrack(previous.track, partial);

      /*
       * Reference identity is the append test, not the length: once the track
       * ring is full every append also evicts, so the length stops changing
       * exactly when a flight is long enough for the odometer to matter.
       */
      const appended = track !== previous.track;

      return {
        ...previous,
        lastSeenMs: receivedAtMs,
        sample: { ...previous.sample, ...partial },
        track,
        odometerM: previous.odometerM + (appended ? lastLegM(track) : 0),
        firstFixAtMs: appended ? (previous.firstFixAtMs ?? observedMs) : previous.firstFixAtMs,
        familySeenMs: { ...previous.familySeenMs, [partial.sourceMessage]: observedMs },
      };
    },
  };
}

/** Folds heartbeat identity into the resolver-facing sample. */
function mergeHeartbeat(
  sample: TelemetrySample,
  heartbeat: HeartbeatState | undefined,
): TelemetrySample {
  if (heartbeat === undefined) {
    return sample;
  }

  return {
    ...sample,
    autopilot: heartbeat.autopilot,
    vehicleType: heartbeat.type,
    customMode: heartbeat.customMode,
    systemStatus: heartbeat.systemStatus,
    armed: heartbeat.armed,
  };
}

function emptyVehicle(sysId: number, compId: number, atMs: number): VehicleView {
  return {
    key: vehicleKey(sysId, compId),
    sysId,
    compId,
    lifecycle: undefined,
    heartbeat: undefined,
    lastFleetAtMs: undefined,
    sample: { sourceMessage: 'NONE', receivedAtMs: atMs },
    track: [],
    odometerM: 0,
    firstFixAtMs: undefined,
    familySeenMs: {},
    lastSeenMs: atMs,
  };
}

function withVehicle(state: FleetState, update: VehicleUpdate | null): FleetState {
  if (update === null) {
    return state;
  }

  const key = vehicleKey(update.sysId, update.compId);
  const previous =
    state.vehicles[key] ?? emptyVehicle(update.sysId, update.compId, Number.NEGATIVE_INFINITY);

  const vehicles = { ...state.vehicles, [key]: update.apply(previous) };
  const order = sortKeys(Object.values(vehicles));

  const pinnedStillExists =
    state.selectionPinned && state.selected !== null && state.selected in vehicles;

  return {
    ...state,
    vehicles,
    order,
    selected: pinnedStillExists ? state.selected : autoSelect(vehicles, order),
    selectionPinned: pinnedStillExists,
  };
}

function sortKeys(views: VehicleView[]): VehicleKey[] {
  return views
    .slice()
    .sort((a, b) => (a.sysId === b.sysId ? a.compId - b.compId : a.sysId - b.sysId))
    .map((view) => view.key);
}

/**
 * Picks the lowest-numbered active vehicle, falling back to the lowest-numbered
 * one when the whole fleet is lost.
 *
 * Deterministic by identity rather than by arrival, so two browsers opened at
 * different moments show the same aircraft, and a reload does not silently
 * change which vehicle the operator is reading.
 */
function autoSelect(
  vehicles: Record<VehicleKey, VehicleView>,
  order: readonly VehicleKey[],
): VehicleKey | null {
  for (const key of order) {
    const view = vehicles[key];

    if (view !== undefined && isActive(view)) {
      return key;
    }
  }

  return order[0] ?? null;
}
