CREATE TABLE IF NOT EXISTS releases (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version     TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'production',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(project_id, version, environment)
);
