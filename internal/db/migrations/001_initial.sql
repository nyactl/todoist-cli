CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '',
    ord         INTEGER NOT NULL DEFAULT 0,
    is_archived INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0, 1)),
    is_favorite INTEGER NOT NULL DEFAULT 0 CHECK (is_favorite IN (0, 1)),
    view_style  TEXT NOT NULL DEFAULT 'list'
) STRICT;

CREATE TABLE IF NOT EXISTS labels (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    color       TEXT NOT NULL DEFAULT '',
    ord         INTEGER NOT NULL DEFAULT 0,
    is_favorite INTEGER NOT NULL DEFAULT 0 CHECK (is_favorite IN (0, 1))
) STRICT;

CREATE TABLE IF NOT EXISTS sections (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ord         INTEGER NOT NULL DEFAULT 0,
    is_archived INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0, 1))
) STRICT;

CREATE TABLE IF NOT EXISTS tasks (
    id               TEXT PRIMARY KEY,
    content          TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    project_id       TEXT REFERENCES projects(id) ON DELETE SET NULL,
    section_id       TEXT REFERENCES sections(id) ON DELETE SET NULL,
    parent_id        TEXT REFERENCES tasks(id)    ON DELETE SET NULL,
    priority         INTEGER NOT NULL DEFAULT 1 CHECK (priority BETWEEN 1 AND 4),
    ord              INTEGER NOT NULL DEFAULT 0,
    is_completed     INTEGER NOT NULL DEFAULT 0 CHECK (is_completed IN (0, 1)),
    due_date         TEXT,
    due_datetime     TEXT,
    due_string       TEXT,
    due_is_recurring INTEGER NOT NULL DEFAULT 0 CHECK (due_is_recurring IN (0, 1)),
    due_timezone     TEXT,
    url              TEXT NOT NULL DEFAULT '',
    comment_count    INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT ''
) STRICT;

CREATE TABLE IF NOT EXISTS task_labels (
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    label_name TEXT NOT NULL,
    PRIMARY KEY (task_id, label_name)
) STRICT;

CREATE TABLE IF NOT EXISTS sync_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_tasks_project
    ON tasks (project_id, ord) WHERE is_completed = 0;

CREATE INDEX IF NOT EXISTS idx_tasks_due
    ON tasks (due_date) WHERE is_completed = 0 AND due_date IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_parent
    ON tasks (parent_id) WHERE parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_section
    ON tasks (section_id) WHERE section_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_task_labels_name
    ON task_labels (label_name);

CREATE INDEX IF NOT EXISTS idx_sections_project
    ON sections (project_id, ord);
