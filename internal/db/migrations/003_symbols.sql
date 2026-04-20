-- 003_symbols.sql: Symbol file storage and symbolication cache

CREATE TABLE IF NOT EXISTS symbol_files (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    release     TEXT NOT NULL DEFAULT '',
    debug_id    TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'breakpad' CHECK(type IN ('breakpad','dwarf','dsym','pdb')),
    filepath    TEXT NOT NULL,
    size        INTEGER NOT NULL DEFAULT 0,
    uploaded_at INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS symbols_cache (
    addr        TEXT NOT NULL,
    debug_id    TEXT NOT NULL,
    module      TEXT NOT NULL DEFAULT '',
    function    TEXT NOT NULL DEFAULT '??',
    file        TEXT NOT NULL DEFAULT '',
    line        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (addr, debug_id)
);

CREATE INDEX IF NOT EXISTS idx_symbols_project ON symbol_files(project_id);
CREATE INDEX IF NOT EXISTS idx_symbols_debug_id ON symbol_files(debug_id);
