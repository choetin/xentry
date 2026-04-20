-- 002_crashes.sql: Crash/issue tracking schema

CREATE TABLE IF NOT EXISTS issues (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    level       TEXT NOT NULL DEFAULT 'error' CHECK(level IN ('fatal','error','warning','info')),
    status      TEXT NOT NULL DEFAULT 'unresolved' CHECK(status IN ('unresolved','resolved','muted')),
    type        TEXT NOT NULL DEFAULT 'crash' CHECK(type IN ('crash','error','assertion')),
    last_seen   INTEGER NOT NULL DEFAULT (unixepoch()),
    first_seen  INTEGER NOT NULL DEFAULT (unixepoch()),
    count       INTEGER NOT NULL DEFAULT 1,
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(project_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_fingerprint ON issues(project_id, fingerprint);

CREATE TABLE IF NOT EXISTS events (
    id               TEXT PRIMARY KEY,
    project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    issue_id         TEXT REFERENCES issues(id) ON DELETE SET NULL,
    release          TEXT NOT NULL DEFAULT '',
    environment      TEXT NOT NULL DEFAULT 'production',
    platform         TEXT NOT NULL DEFAULT '',
    timestamp        INTEGER NOT NULL DEFAULT (unixepoch()),
    message          TEXT NOT NULL DEFAULT '',
    payload          TEXT NOT NULL DEFAULT '{}',
    deleted_at       INTEGER NOT NULL DEFAULT 0,
    stackwalk_output TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_events_project ON events(project_id);
CREATE INDEX IF NOT EXISTS idx_events_issue ON events(issue_id);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

CREATE TABLE IF NOT EXISTS threads (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    crashed     INTEGER NOT NULL DEFAULT 0,
    frame_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS frames (
    id           TEXT PRIMARY KEY,
    thread_id    TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    frame_no     INTEGER NOT NULL,
    function     TEXT NOT NULL DEFAULT '',
    file         TEXT NOT NULL DEFAULT '',
    line         INTEGER NOT NULL DEFAULT 0,
    addr         TEXT NOT NULL DEFAULT '0x0',
    module       TEXT NOT NULL DEFAULT '',
    symbolicated INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_frames_thread ON frames(thread_id);
