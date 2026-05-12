package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Task struct {
	ID          string
	Content     string
	ProjectID   string
	ProjectName string
	SectionID   string
	SectionName string
	Priority    int
	Order       int
	DueDate     string
	DueDatetime string
}

const selectCols = `
	SELECT t.id, t.content,
	       COALESCE(t.project_id, ''), COALESCE(p.name, ''),
	       COALESCE(t.section_id, ''), COALESCE(s.name, ''),
	       t.priority, t.ord,
	       COALESCE(t.due_date, ''), COALESCE(t.due_datetime, '')
	FROM tasks t
	LEFT JOIN projects p ON t.project_id = p.id
	LEFT JOIN sections s ON t.section_id = s.id`

func DueToday(ctx context.Context, db *sql.DB) ([]Task, error) {
	return query(ctx, db,
		selectCols+`
		WHERE t.is_completed = 0
		  AND t.due_date IS NOT NULL
		  AND t.due_date <= date('now')
		  AND (t.parent_id IS NULL OR t.parent_id = '')
		ORDER BY t.due_date ASC, t.priority DESC, p.ord, t.ord`)
}

func ByProject(ctx context.Context, db *sql.DB, projectID string) ([]Task, error) {
	return query(ctx, db,
		selectCols+`
		WHERE t.is_completed = 0
		  AND t.project_id = ?
		  AND (t.parent_id IS NULL OR t.parent_id = '')
		ORDER BY COALESCE(s.ord, 0), t.ord`,
		projectID)
}

func ByLabels(ctx context.Context, db *sql.DB, labelNames []string, projectID string) ([]Task, error) {
	if len(labelNames) == 0 {
		return nil, fmt.Errorf("at least one label required")
	}
	placeholders := make([]string, len(labelNames))
	args := make([]any, len(labelNames))
	for i, l := range labelNames {
		placeholders[i] = "?"
		args[i] = l
	}
	subquery := fmt.Sprintf(
		`SELECT task_id FROM task_labels WHERE label_name IN (%s)
		 GROUP BY task_id HAVING COUNT(DISTINCT label_name) = %d`,
		strings.Join(placeholders, ","), len(labelNames))

	where := fmt.Sprintf(`
		WHERE t.is_completed = 0
		  AND (t.parent_id IS NULL OR t.parent_id = '')
		  AND t.id IN (%s)`, subquery)

	if projectID != "" {
		where += " AND t.project_id = ?"
		args = append(args, projectID)
	}
	return query(ctx, db,
		selectCols+where+` ORDER BY t.priority DESC, p.ord, t.ord`,
		args...)
}

func ByID(ctx context.Context, db *sql.DB, idOrPrefix string) (*Task, error) {
	ts, err := query(ctx, db, selectCols+` WHERE t.id = ?`, idOrPrefix)
	if err != nil {
		return nil, err
	}
	if len(ts) == 1 {
		return &ts[0], nil
	}
	ts, err = query(ctx, db, selectCols+` WHERE t.id LIKE ?`, idOrPrefix+"%")
	if err != nil {
		return nil, err
	}
	if len(ts) == 1 {
		return &ts[0], nil
	}
	if len(ts) > 1 {
		return nil, fmt.Errorf("prefix %q is ambiguous (%d matches) — use more characters", idOrPrefix, len(ts))
	}
	// fall back to exact content match (supports completion by task name)
	ts, err = query(ctx, db, selectCols+` WHERE t.content = ?`, idOrPrefix)
	if err != nil {
		return nil, err
	}
	switch len(ts) {
	case 0:
		return nil, fmt.Errorf("task %q not found in local cache — run: todoist-cli sync", idOrPrefix)
	case 1:
		return &ts[0], nil
	default:
		return nil, fmt.Errorf("task content %q is ambiguous (%d matches) — use an ID instead", idOrPrefix, len(ts))
	}
}

func Subtasks(ctx context.Context, db *sql.DB, taskID string) ([]Task, error) {
	return query(ctx, db,
		selectCols+`
		WHERE t.is_completed = 0
		  AND t.parent_id = ?
		ORDER BY t.ord`,
		taskID)
}

func Labels(ctx context.Context, db *sql.DB, taskID string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT label_name FROM task_labels WHERE task_id = ? ORDER BY label_name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func ProjectExists(ctx context.Context, db *sql.DB, projectID string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM projects WHERE id = ?`, projectID).Scan(&n)
	return n > 0, err
}

func SectionByName(ctx context.Context, db *sql.DB, name, projectID string) (id string, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT id FROM sections WHERE name = ? AND project_id = ?`, name, projectID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("section %q not found in project — run: todoist-cli sync", name)
	}
	return id, err
}

func ProjectByName(ctx context.Context, db *sql.DB, name string) (id string, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT id FROM projects WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("project %q not found in local cache — run: todoist-cli sync", name)
	}
	return id, err
}

func query(ctx context.Context, q *sql.DB, sql string, args ...any) ([]Task, error) {
	rows, err := q.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()
	var ts []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(
			&t.ID, &t.Content,
			&t.ProjectID, &t.ProjectName,
			&t.SectionID, &t.SectionName,
			&t.Priority, &t.Order,
			&t.DueDate, &t.DueDatetime,
		); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, rows.Err()
}
