CREATE TABLE recording_events_v2 (
    recording_id INTEGER NOT NULL REFERENCES recordings(id),
    seq          INTEGER NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('fleet', 'telemetry', 'command')),
    occurred_at  INTEGER NOT NULL,
    payload      BLOB NOT NULL,
    PRIMARY KEY (recording_id, seq)
) WITHOUT ROWID;

INSERT INTO recording_events_v2 SELECT * FROM recording_events;
DROP TABLE recording_events;
ALTER TABLE recording_events_v2 RENAME TO recording_events;
