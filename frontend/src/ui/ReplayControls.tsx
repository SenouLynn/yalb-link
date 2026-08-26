/** Transport controls for a recorded flight. */

import { useEffect, useState, useSyncExternalStore } from 'react';

import {
  deleteRecording,
  RECORDINGS_PATH,
  RecordingHTTPError,
  type RecordingWire,
  type ReplayEventSource,
} from '@/stream/replay';
import { replayUrl } from '@/stream/select';

import { NO_VALUE } from './format';

/** Offered playback speeds. Real time first, because it is the honest one. */
const SPEEDS = [1, 2, 4, 8];

export interface ReplayControlsProps {
  /** The running replay, or null when no recording has been chosen. */
  source: ReplayEventSource | null;
}

export function ReplayControls({ source }: ReplayControlsProps) {
  if (source === null) {
    return <RecordingPicker />;
  }

  return <Transport source={source} />;
}

function Transport({ source }: { source: ReplayEventSource }) {
  // Both arguments must be stable references: `subscribe` is a bound field on
  // the source and `status` is cached there, so React neither resubscribes nor
  // re-renders unless playback actually moved.
  const status = useSyncExternalStore(source.subscribe, source.status);

  const spanMs = status.endedAtMs - status.startedAtMs;
  const offsetMs = status.positionMs - status.startedAtMs;

  if (status.error !== null) {
    return (
      <div className="panel replay replay--error">
        <span className="label">Replay unavailable</span>
        <span>{status.error}</span>
      </div>
    );
  }

  return (
    <div className="panel replay">
      <button
        type="button"
        className="replay__button"
        onClick={() => {
          if (status.playing) {
            source.pause();
          } else {
            source.play();
          }
        }}
        disabled={status.loading || status.totalEvents === 0}
      >
        {status.playing ? 'Pause' : 'Play'}
      </button>

      <span className="replay__clock">
        {formatOffset(offsetMs)} / {formatOffset(spanMs)}
      </span>

      <input
        type="range"
        className="replay__scrub"
        min={0}
        max={Math.max(spanMs, 1)}
        step={100}
        value={Math.round(offsetMs)}
        aria-label="Playback position"
        onChange={(change) => {
          source.seekTo(status.startedAtMs + Number(change.target.value));
        }}
      />

      <label className="replay__speed">
        <span className="label">Speed</span>
        <select
          value={status.speed}
          onChange={(change) => {
            source.setSpeed(Number(change.target.value));
          }}
        >
          {SPEEDS.map((speed) => (
            <option key={speed} value={speed}>
              {speed}×
            </option>
          ))}
        </select>
      </label>

      <span className="replay__name">{status.recording?.name ?? NO_VALUE}</span>

      {status.loading && <span className="replay__note">loading…</span>}
      {status.truncated && (
        <span className="replay__note replay__note--warn">
          first {status.totalEvents} events only
        </span>
      )}
    </div>
  );
}

/** Lists recordings so one can be opened. Shown when none is selected. */
function RecordingPicker() {
  const [recordings, setRecordings] = useState<RecordingWire[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const response = await globalThis.fetch(RECORDINGS_PATH);

        if (response.status === 404) {
          // The routes exist only when recording is enabled, so a 404 here is
          // a configuration answer rather than a missing recording.
          throw new Error('Recording is disabled on this backend (GCS_RECORDING_ENABLED).');
        }
        if (!response.ok) {
          throw new Error(`The backend returned HTTP ${String(response.status)}.`);
        }
        const listed = (await response.json()) as RecordingWire[];

        if (!cancelled) {
          setRecordings(listed);
        }
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="panel replay">
      <span className="label">Replay</span>
      {error !== null && <span className="replay__note replay__note--warn">{error}</span>}
      {error === null && recordings === null && <span className="replay__note">loading…</span>}
      {error === null && recordings?.length === 0 && (
        <span className="replay__note">No recordings yet.</span>
      )}
      {recordings?.map((recording) => (
        <div key={recording.id} className="replay__pick-row">
          <a className="replay__pick" href={replayUrl(recording.id)}>
            {recording.name === '' ? `#${String(recording.id)}` : recording.name}
            <span className="label">
              {recording.event_count} events · {recording.status}
            </span>
          </a>
          <button
            type="button"
            className="replay__delete"
            disabled={deleting === recording.id}
            aria-label={`Delete ${recording.name === '' ? `recording ${String(recording.id)}` : recording.name}`}
            onClick={() => {
              const label = recording.name === '' ? `recording #${String(recording.id)}` : recording.name;
              if (!globalThis.confirm(`Permanently delete ${label}?`)) {
                return;
              }
              setDeleting(recording.id);
              setError(null);
              void deleteRecording(recording.id)
                .then(() => {
                  setRecordings((current) => current?.filter((item) => item.id !== recording.id) ?? []);
                })
                .catch((cause: unknown) => {
                  if (cause instanceof RecordingHTTPError && cause.status === 409) {
                    setError('Stop the recording first.');
                  } else {
                    setError(cause instanceof Error ? cause.message : String(cause));
                  }
                })
                .finally(() => {
                  setDeleting(null);
                });
            }}
          >
            {deleting === recording.id ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      ))}
    </div>
  );
}

/** Elapsed time into the recording, as m:ss. */
export function formatOffset(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return NO_VALUE;
  }

  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${String(minutes)}:${String(seconds).padStart(2, '0')}`;
}
