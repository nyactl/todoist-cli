ALTER TABLE projects ADD COLUMN parent_id TEXT;

CREATE INDEX IF NOT EXISTS idx_projects_parent
    ON projects (parent_id) WHERE parent_id IS NOT NULL;
