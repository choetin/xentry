CREATE TABLE IF NOT EXISTS logs (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    timestamp   INTEGER NOT NULL DEFAULT (unixepoch()),
    level       TEXT NOT NULL DEFAULT 'info' CHECK(level IN ('debug','info','warn','error','fatal')),
    message     TEXT NOT NULL DEFAULT '',
    logger      TEXT NOT NULL DEFAULT '',
    trace_id    TEXT NOT NULL DEFAULT '',
    span_id     TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL DEFAULT 'production',
    release     TEXT NOT NULL DEFAULT '',
    attributes  TEXT NOT NULL DEFAULT '{}',
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_logs_project ON logs(project_id);
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(project_id, level);

CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts USING fts5(
    message,
    logger,
    content=logs,
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS logs_ai AFTER INSERT ON logs BEGIN
    INSERT INTO logs_fts(rowid, message, logger) VALUES (new.rowid, new.message, new.logger);
END;
