/** Transport controls for a recorded flight. */

import { Group, Field, Note, Lever } from '@/ui/primitives';

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
      <Group label="Replay unavailable" className="replay"><Note tone="caution">{status.error}</Note></Group>
    );
  }

  return (
    <Group label="Replay" className="replay">
      <Lever
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
      </Lever>

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

      <Field label="Speed" value={String(status.speed)}
        onChange={(value) => { source.setSpeed(Number(value)); }}
        options={SPEEDS.map((speed) => ({ value: String(speed), label: `${String(speed)}×` }))} />

      <span className="replay__name">{status.recording?.name ?? NO_VALUE}</span>

      {status.loading && <Note>loading…</Note>}
      {status.truncated && (
        <Note tone="caution">
          first {status.totalEvents} events only
        </Note>
      )}
    </Group>
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
    <Group label="Replay" className="replay">
      {error !== null && <Note tone="caution">{error}</Note>}
      {error === null && recordings === null && <Note>loading…</Note>}
      {error === null && recordings?.length === 0 && (
        <Note>No recordings yet.</Note>
      )}
      {recordings?.map((recording) => (
        <div key={recording.id} className="replay__pick-row">
          <a className="lever" href={replayUrl(recording.id)}>
            {recording.name === '' ? `#${String(recording.id)}` : recording.name}
            <span className="replay__name">
              {recording.event_count} events · {recording.status}
            </span>
          </a>
          <Lever
            caution
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
          </Lever>
        </div>
      ))}
    </Group>
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
