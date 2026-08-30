-- =========================================================
-- Bugsbunny — Corrected Schema (P2: Bug Management & Workflow)
-- This file is the single source of truth for the DB.
-- Run this on a fresh database before starting the Go backend:
--   docker exec -i bugsbunny-db psql -U postgres -d bugsbunny < schema.sql
--   docker exec -i bugsbunny-db psql -U postgres -d bugsbunny < seed.sql
-- =========================================================

-- 1. Users
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT 'developer',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Projects
CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    project_key VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT DEFAULT '',
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    issue_counter INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Issues (the engine core)
-- Priority = urgency, Severity = impact. Kept as two distinct columns on purpose.
CREATE TABLE issues (
    id SERIAL PRIMARY KEY,
    issue_key VARCHAR(50) UNIQUE NOT NULL,
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT DEFAULT '',

    status VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open','triaged','in_progress','resolved','verified','closed')),

    priority VARCHAR(20) NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low','medium','high','critical')),

    severity VARCHAR(20) NOT NULL DEFAULT 'minor'
        CHECK (severity IN ('minor','major','critical','blocker')),

    component VARCHAR(100) DEFAULT '',
    environment VARCHAR(100) DEFAULT '',
    module VARCHAR(100) DEFAULT '',
    release VARCHAR(50) DEFAULT '',

    reporter_id INTEGER REFERENCES users(id),
    assignee_id INTEGER REFERENCES users(id),

    reopen_count INTEGER NOT NULL DEFAULT 0,

    -- Free-form extension bucket (kept for forward-compatibility; core fields
    -- above are real columns now, not stuffed into JSON like the old build).
    custom_data JSONB DEFAULT '{}',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_issues_status ON issues(status);
CREATE INDEX idx_issues_project ON issues(project_id);
CREATE INDEX idx_issues_assignee ON issues(assignee_id);
CREATE INDEX idx_issues_custom_data ON issues USING GIN (custom_data);

-- 4. Comments
CREATE TABLE issue_comments (
    id SERIAL PRIMARY KEY,
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    body TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. Attachments
CREATE TABLE issue_attachments (
    id SERIAL PRIMARY KEY,
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    filename VARCHAR(255) NOT NULL,
    file_url VARCHAR(500) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 6. Labels (reusable per project)
CREATE TABLE labels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (name, project_id)
);

-- 7. Issue <-> Label join table
CREATE TABLE issue_labels (
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    label_id INTEGER REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_key, label_id)
);

-- 8. Issue relationships (related_to / duplicate_of / blocks / blocked_by)
CREATE TABLE issue_relationships (
    id SERIAL PRIMARY KEY,
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    related_issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    relation_type VARCHAR(20) NOT NULL
        CHECK (relation_type IN ('related_to','duplicate_of','blocks','blocked_by')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (issue_key <> related_issue_key),
    UNIQUE (issue_key, related_issue_key, relation_type)
);

-- 9. Watchers (follow/unfollow)
CREATE TABLE issue_watchers (
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_key, user_id)
);

-- 10. Activity stream (audit trail)
CREATE TABLE activity_stream (
    id SERIAL PRIMARY KEY,
    issue_key VARCHAR(50) REFERENCES issues(issue_key) ON DELETE CASCADE,
    actor_id INTEGER REFERENCES users(id),
    action_type VARCHAR(50) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activity_issue ON activity_stream(issue_key);
