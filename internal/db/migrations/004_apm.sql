CREATE TABLE IF NOT EXISTS transactions (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    trace_id    TEXT NOT NULL DEFAULT '',
    span_id     TEXT NOT NULL DEFAULT '',
    parent_id   TEXT NOT NULL DEFAULT '',
    op          TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ok' CHECK(status IN ('ok','error','deadline_exceeded','cancelled')),
    environment TEXT NOT NULL DEFAULT 'production',
    release     TEXT NOT NULL DEFAULT '',
    start_time  REAL NOT NULL DEFAULT 0,
    duration    REAL NOT NULL DEFAULT 0,
    timestamp   INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS spans (
    id          TEXT PRIMARY KEY,
    tx_id       TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    parent_id   TEXT NOT NULL DEFAULT '',
    span_id     TEXT NOT NULL DEFAULT '',
    op          TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ok',
    start_time  REAL NOT NULL DEFAULT 0,
    duration    REAL NOT NULL DEFAULT 0,
    tags        TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_tx_project ON transactions(project_id);
CREATE INDEX IF NOT EXISTS idx_tx_timestamp ON transactions(timestamp);
CREATE INDEX IF NOT EXISTS idx_tx_name ON transactions(project_id, name);
CREATE INDEX IF NOT EXISTS idx_spans_tx ON spans(tx_id);
