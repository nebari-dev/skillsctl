-- +goose Up
CREATE TABLE skills (
    name            TEXT PRIMARY KEY,
    description     TEXT NOT NULL,
    owner           TEXT NOT NULL,
    tags            TEXT NOT NULL DEFAULT '[]',
    latest_version  TEXT NOT NULL,
    install_count   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    source          INTEGER NOT NULL,
    marketplace_id  TEXT NOT NULL DEFAULT '',
    upstream_url    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_skills_updated ON skills(updated_at DESC);

CREATE TABLE skill_versions (
    skill_name    TEXT NOT NULL REFERENCES skills(name) ON DELETE CASCADE,
    version       TEXT NOT NULL,
    oci_ref       TEXT NOT NULL DEFAULT '',
    digest        TEXT NOT NULL DEFAULT '',
    published_by  TEXT NOT NULL DEFAULT '',
    changelog     TEXT NOT NULL DEFAULT '',
    size_bytes    INTEGER NOT NULL DEFAULT 0,
    draft         INTEGER NOT NULL DEFAULT 0,
    published_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (skill_name, version)
);

-- +goose Down
DROP TABLE IF EXISTS skill_versions;
DROP TABLE IF EXISTS skills;
