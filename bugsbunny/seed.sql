-- =========================================================
-- Bugsbunny — Seed Data (matches schema.sql)
-- Run AFTER schema.sql.
-- Note: main.go automatically sets bugs_admin's password to
-- "admin123" on server startup. The other seeded users have no
-- password set yet — set one via POST /users or the admin panel
-- before trying to log in as them.
-- =========================================================

-- 1. Users
INSERT INTO users (email, role, is_active) VALUES
('admin@bugsbunny.local', 'admin', TRUE),
('daffy@bugsbunny.local', 'developer', TRUE),
('lola@bugsbunny.local', 'qa', TRUE);

-- 2. Project
INSERT INTO projects (project_key, name, description, issue_counter) VALUES
('BUNNY', 'Bugsbunny Core Engine', 'The core issue tracker engine itself.', 2);

-- 3. Labels
INSERT INTO labels (name, project_id)
SELECT 'architecture', id FROM projects WHERE project_key = 'BUNNY' UNION ALL
SELECT 'wasm', id FROM projects WHERE project_key = 'BUNNY' UNION ALL
SELECT 'ui', id FROM projects WHERE project_key = 'BUNNY' UNION ALL
SELECT 'regression', id FROM projects WHERE project_key = 'BUNNY';

-- 4. Issues
INSERT INTO issues (issue_key, project_id, title, body, status, priority, severity, component, reporter_id, assignee_id)
VALUES
(
    'BUNNY-1',
    (SELECT id FROM projects WHERE project_key = 'BUNNY'),
    'Design WASM plugin architecture',
    'We need to finalize how community plugins will interface with the core engine.',
    'in_progress', 'critical', 'major', 'backend',
    (SELECT id FROM users WHERE email = 'admin@bugsbunny.local'),
    (SELECT id FROM users WHERE email = 'daffy@bugsbunny.local')
),
(
    'BUNNY-2',
    (SELECT id FROM projects WHERE project_key = 'BUNNY'),
    'Dark mode toggle missing on dashboard',
    'The dark mode toggle is completely hidden on Safari.',
    'open', 'medium', 'minor', 'frontend',
    (SELECT id FROM users WHERE email = 'lola@bugsbunny.local'),
    NULL
);

-- 5. Issue <-> Label links
INSERT INTO issue_labels (issue_key, label_id)
SELECT 'BUNNY-1', id FROM labels WHERE name = 'architecture' UNION ALL
SELECT 'BUNNY-1', id FROM labels WHERE name = 'wasm' UNION ALL
SELECT 'BUNNY-2', id FROM labels WHERE name = 'ui';

-- 6. Activity stream (audit log)
INSERT INTO activity_stream (issue_key, actor_id, action_type, old_value, new_value)
VALUES
(
    'BUNNY-1',
    (SELECT id FROM users WHERE email = 'daffy@bugsbunny.local'),
    'status_changed',
    'triaged',
    'in_progress'
),
(
    'BUNNY-2',
    (SELECT id FROM users WHERE email = 'lola@bugsbunny.local'),
    'comment_added',
    NULL,
    'Confirmed this is still broken in version 1.0.2'
);

-- 7. A sample comment
INSERT INTO issue_comments (issue_key, user_id, body)
VALUES (
    'BUNNY-2',
    (SELECT id FROM users WHERE email = 'lola@bugsbunny.local'),
    'Confirmed this is still broken in version 1.0.2'
);
