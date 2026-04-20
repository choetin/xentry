-- 001_init.sql: Core schema — users, organizations, projects, tokens

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    email       TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS organizations (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS org_members (
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner','admin','member')),
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    platform    TEXT NOT NULL DEFAULT 'other' CHECK(platform IN ('windows','macos','linux','android','ios','other')),
    dsn_token   TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(org_id, slug)
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    scopes      TEXT NOT NULL DEFAULT 'event:write',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    deleted_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_project ON api_tokens(project_id);
