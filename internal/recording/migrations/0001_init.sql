CREATE TABLE recordings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    started_at  INTEGER NOT NULL,
    stopped_at  INTEGER,
    status      TEXT NOT NULL CHECK (status IN ('active', 'stopped', 'limit_reached', 'error')),
    stop_reason TEXT
);

CREATE TABLE recording_events (
    recording_id INTEGER NOT NULL REFERENCES recordings(id),
    seq          INTEGER NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('fleet', 'telemetry')),
    occurred_at  INTEGER NOT NULL,
    payload      BLOB NOT NULL,
    PRIMARY KEY (recording_id, seq)
) WITHOUT ROWID;
