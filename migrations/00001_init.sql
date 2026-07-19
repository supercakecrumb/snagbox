-- +goose Up
CREATE TABLE users (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    name        TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    token_hash   TEXT NOT NULL UNIQUE, -- hex SHA-256 of the bearer token
    created_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ
);

-- A row with project_id NULL grants the agent access to the inbox
-- (issues that have no project).
CREATE TABLE agent_project_perms (
    agent_id   BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_id BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE NULLS NOT DISTINCT (agent_id, project_id)
);

CREATE TABLE ingest_tokens (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE, -- hex SHA-256 of the bearer token
    label      TEXT NOT NULL,
    project_id BIGINT REFERENCES projects(id) ON DELETE CASCADE, -- NULL = files into inbox
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE issues (
    id             UUID PRIMARY KEY, -- UUIDv7, generated in the app
    project_id     BIGINT REFERENCES projects(id) ON DELETE SET NULL, -- NULL = inbox
    source         TEXT NOT NULL CHECK (source IN ('telegram','api')),
    author_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    body           TEXT NOT NULL DEFAULT '',
    meta           JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX issues_project_created_idx ON issues (project_id, created_at);
CREATE INDEX issues_created_idx ON issues (created_at);

CREATE TABLE attachments (
    id         UUID PRIMARY KEY,
    issue_id   UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    s3_key     TEXT NOT NULL,
    mime       TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    filename   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX attachments_issue_idx ON attachments (issue_id);

-- Fan-out queue state: an issue is "pending" for an agent until that agent
-- acks it. Queue = permitted issues with no ack row for the agent.
CREATE TABLE acks (
    agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    acked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, issue_id)
);

-- msgr-authkit storage (bot-first web login), adapted to Postgres.
CREATE TABLE auth_intents (
    id               TEXT PRIMARY KEY,
    code             TEXT NOT NULL,
    messenger        TEXT NOT NULL,
    audience         TEXT NOT NULL DEFAULT '',
    subject_id       TEXT NOT NULL DEFAULT '',
    state            TEXT NOT NULL,
    identity_json    TEXT,
    metadata_json    TEXT,
    redemption_mode  TEXT NOT NULL,
    max_redemptions  INTEGER NOT NULL DEFAULT 0,
    redemption_count INTEGER NOT NULL DEFAULT 0,
    expires_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL,
    consumed_at      TIMESTAMPTZ,
    UNIQUE (messenger, code)
);

CREATE TABLE account_links (
    app_user_id       TEXT NOT NULL,
    messenger         TEXT NOT NULL,
    messenger_user_id TEXT NOT NULL,
    username          TEXT NOT NULL DEFAULT '',
    name              TEXT NOT NULL DEFAULT '',
    surname           TEXT NOT NULL DEFAULT '',
    birth_date        TIMESTAMPTZ,
    attributes_json   TEXT,
    linked_at         TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (messenger, messenger_user_id)
);

-- Web session tokens are stored as SHA-256 hashes, never in plaintext.
CREATE TABLE web_sessions (
    session_id TEXT PRIMARY KEY,
    subject_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    issued_at  TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE web_sessions;
DROP TABLE account_links;
DROP TABLE auth_intents;
DROP TABLE acks;
DROP TABLE attachments;
DROP TABLE issues;
DROP TABLE ingest_tokens;
DROP TABLE agent_project_perms;
DROP TABLE agents;
DROP TABLE projects;
DROP TABLE users;
