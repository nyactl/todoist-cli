package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var syncProject string

var syncCmd = &cobra.Command{
	Use:               "sync",
	Short:             "Pull latest data from Todoist into local cache",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx := cmd.Context()
		client := todoist.New(token)

		if syncProject != "" {
			return runProjectSync(cmd, ctx, conn, client, syncProject)
		}
		return runFullSync(cmd, ctx, conn, client)
	},
}

func runFullSync(cmd *cobra.Command, ctx context.Context, conn *sql.DB, client *todoist.Client) error {
	type fetchResult struct {
		labels   []todoist.Label
		projects []todoist.Project
		sections []todoist.Section
		tasks    []todoist.Task
	}
	var (
		fetched  fetchResult
		mu       sync.Mutex
		wg       sync.WaitGroup
		fetchErr error
	)
	setErr := func(err error) {
		mu.Lock()
		if fetchErr == nil {
			fetchErr = err
		}
		mu.Unlock()
	}
	wg.Add(4)
	go func() {
		defer wg.Done()
		items, err := client.GetLabels(ctx)
		if err != nil {
			setErr(fmt.Errorf("labels: %w", err))
			return
		}
		mu.Lock()
		fetched.labels = items
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items, err := client.GetProjects(ctx)
		if err != nil {
			setErr(fmt.Errorf("projects: %w", err))
			return
		}
		mu.Lock()
		fetched.projects = items
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items, err := client.GetSections(ctx, "")
		if err != nil {
			setErr(fmt.Errorf("sections: %w", err))
			return
		}
		mu.Lock()
		fetched.sections = items
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items, err := client.GetTasks(ctx, "")
		if err != nil {
			setErr(fmt.Errorf("tasks: %w", err))
			return
		}
		mu.Lock()
		fetched.tasks = items
		mu.Unlock()
	}()
	wg.Wait()
	if fetchErr != nil {
		return fmt.Errorf("sync fetch: %w", fetchErr)
	}

	type step struct {
		name string
		fn   func(context.Context, *sql.DB) (int, error)
	}
	for _, s := range []step{
		{"labels", func(ctx context.Context, db *sql.DB) (int, error) { return writeLabels(ctx, db, fetched.labels) }},
		{"projects", func(ctx context.Context, db *sql.DB) (int, error) { return writeProjects(ctx, db, fetched.projects) }},
		{"sections", func(ctx context.Context, db *sql.DB) (int, error) {
			return writeSections(ctx, db, fetched.sections, "")
		}},
		{"tasks", func(ctx context.Context, db *sql.DB) (int, error) { return writeTasks(ctx, db, fetched.tasks, "") }},
	} {
		n, err := s.fn(ctx, conn)
		if err != nil {
			return fmt.Errorf("sync %s: %w", s.name, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %d\n", s.name, n)
	}

	_, err := conn.ExecContext(ctx,
		`INSERT INTO sync_state(key,value) VALUES('last_synced_at',?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func runProjectSync(cmd *cobra.Command, ctx context.Context, conn *sql.DB, client *todoist.Client, project string) error {
	// Resolve project name or ID to a confirmed project ID.
	projectID, err := tasks.ProjectByName(ctx, conn, project)
	if err != nil {
		// Try treating the input as a raw ID.
		ok, err2 := tasks.ProjectExists(ctx, conn, project)
		if err2 != nil || !ok {
			return fmt.Errorf("project %q not found in local cache — run: todoist-cli sync", project)
		}
		projectID = project
	}

	type fetchResult struct {
		sections []todoist.Section
		tasks    []todoist.Task
	}
	var (
		fetched  fetchResult
		mu       sync.Mutex
		wg       sync.WaitGroup
		fetchErr error
	)
	setErr := func(err error) {
		mu.Lock()
		if fetchErr == nil {
			fetchErr = err
		}
		mu.Unlock()
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		items, err := client.GetSections(ctx, projectID)
		if err != nil {
			setErr(fmt.Errorf("sections: %w", err))
			return
		}
		mu.Lock()
		fetched.sections = items
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items, err := client.GetTasks(ctx, projectID)
		if err != nil {
			setErr(fmt.Errorf("tasks: %w", err))
			return
		}
		mu.Lock()
		fetched.tasks = items
		mu.Unlock()
	}()
	wg.Wait()
	if fetchErr != nil {
		return fmt.Errorf("sync fetch: %w", fetchErr)
	}

	type step struct {
		name string
		fn   func(context.Context, *sql.DB) (int, error)
	}
	for _, s := range []step{
		{"sections", func(ctx context.Context, db *sql.DB) (int, error) {
			return writeSections(ctx, db, fetched.sections, projectID)
		}},
		{"tasks", func(ctx context.Context, db *sql.DB) (int, error) {
			return writeTasks(ctx, db, fetched.tasks, projectID)
		}},
	} {
		n, err := s.fn(ctx, conn)
		if err != nil {
			return fmt.Errorf("sync %s: %w", s.name, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %d\n", s.name, n)
	}
	return nil
}

func writeLabels(ctx context.Context, db *sql.DB, items []todoist.Label) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := purgeMissing(ctx, tx, "labels", "", "", ids(items, func(l todoist.Label) string { return l.ID })); err != nil {
		return 0, err
	}
	for _, l := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO labels(id,name,color,ord,is_favorite) VALUES(?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color,
			   ord=excluded.ord, is_favorite=excluded.is_favorite`,
			l.ID, l.Name, l.Color, l.Order, boolToInt(l.IsFavorite))
		if err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit()
}

func writeProjects(ctx context.Context, db *sql.DB, items []todoist.Project) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// Projects deleted or archived elsewhere are not in the list any more;
	// dropping them cascades to their sections and detaches their tasks,
	// which writeTasks then skips (#43).
	if err := purgeMissing(ctx, tx, "projects", "", "", ids(items, func(p todoist.Project) string { return p.ID })); err != nil {
		return 0, err
	}
	for _, p := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO projects(id,name,color,ord,parent_id,is_archived,is_favorite,view_style)
			 VALUES(?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, ord=excluded.ord,
			   parent_id=excluded.parent_id,
			   is_archived=excluded.is_archived, is_favorite=excluded.is_favorite,
			   view_style=excluded.view_style`,
			p.ID, p.Name, p.Color, p.Order, nullIfEmpty(p.ParentID),
			boolToInt(p.IsArchived), boolToInt(p.IsFavorite), p.ViewStyle)
		if err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit()
}

func writeSections(ctx context.Context, db *sql.DB, items []todoist.Section, projectID string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	scopeCol := ""
	if projectID != "" {
		scopeCol = "project_id"
	}
	if err := purgeMissing(ctx, tx, "sections", scopeCol, projectID, ids(items, func(s todoist.Section) string { return s.ID })); err != nil {
		return 0, err
	}

	for i, s := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO sections(id,name,project_id,ord,is_archived) VALUES(?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, project_id=excluded.project_id,
			   ord=excluded.ord, is_archived=excluded.is_archived`,
			s.ID, s.Name, s.ProjectID, i, boolToInt(s.IsArchived))
		if err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit()
}

func writeTasks(ctx context.Context, db *sql.DB, items []todoist.Task, projectID string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	scopeCol := ""
	if projectID != "" {
		scopeCol = "project_id"
	}
	if err := purgeMissing(ctx, tx, "tasks", scopeCol, projectID, ids(items, func(t todoist.Task) string { return t.ID })); err != nil {
		return 0, err
	}
	written, skipped := 0, 0
	for _, t := range items {
		// A task whose project was deleted server-side can keep appearing in the
		// list endpoint for minutes, with a project_id that no longer resolves.
		// Inserting it violates the project_id FK and would abort the entire sync
		// (#12). Skip it — and drop any stale cached copy — rather than failing.
		if t.ProjectID != "" {
			var exists bool
			if err := tx.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, t.ProjectID).Scan(&exists); err != nil {
				return 0, err
			}
			if !exists {
				if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, t.ID); err != nil {
					return 0, err
				}
				skipped++
				continue
			}
		}

		var dueDate, dueDatetime, dueString, dueTZ *string
		dueRecurring := 0
		if t.Due != nil {
			dueDate = &t.Due.Date
			if t.Due.Datetime != "" {
				dueDatetime = &t.Due.Datetime
			}
			if t.Due.String != "" {
				dueString = &t.Due.String
			}
			if t.Due.Timezone != "" {
				dueTZ = &t.Due.Timezone
			}
			dueRecurring = boolToInt(t.Due.IsRecurring)
		}
		var deadline *string
		if t.Deadline != nil && t.Deadline.Date != "" {
			deadline = &t.Deadline.Date
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO tasks(
			   id,content,description,project_id,section_id,parent_id,
			   priority,ord,is_completed,
			   due_date,due_datetime,due_string,due_is_recurring,due_timezone,
			   deadline_date,url,comment_count,created_at)
			 VALUES(?,?,?,nullif(?,?),nullif(?,?),nullif(?,?),?,?,?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   content=excluded.content, description=excluded.description,
			   project_id=excluded.project_id, section_id=excluded.section_id,
			   parent_id=excluded.parent_id, priority=excluded.priority,
			   ord=excluded.ord, is_completed=excluded.is_completed,
			   due_date=excluded.due_date, due_datetime=excluded.due_datetime,
			   due_string=excluded.due_string, due_is_recurring=excluded.due_is_recurring,
			   due_timezone=excluded.due_timezone, deadline_date=excluded.deadline_date,
			   url=excluded.url, comment_count=excluded.comment_count`,
			t.ID, t.Content, t.Description,
			t.ProjectID, "", t.SectionID, "", t.ParentID, "",
			t.Priority, t.Order, boolToInt(t.IsCompleted),
			dueDate, dueDatetime, dueString, dueRecurring, dueTZ,
			deadline, t.URL, t.CommentCount, t.CreatedAt)
		if err != nil {
			return 0, fmt.Errorf("task %s: %w", t.ID, err)
		}
		written++
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM task_labels WHERE task_id=?`, t.ID); err != nil {
			return 0, err
		}
		for _, label := range t.Labels {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO task_labels(task_id,label_name) VALUES(?,?)`,
				t.ID, label); err != nil {
				return 0, err
			}
		}
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "sync: skipped %d task(s) whose project no longer exists (deleted server-side)\n", skipped)
	}
	return written, tx.Commit()
}

// upsertTasks inserts or updates a slice of tasks in the local cache.
// Used by add.go for optimistic local inserts after creating a task.
func upsertTasks(ctx context.Context, db *sql.DB, items []todoist.Task) {
	for _, t := range items {
		var dueDate, dueDatetime, dueString, dueTZ *string
		dueRecurring := 0
		if t.Due != nil {
			dueDate = &t.Due.Date
			if t.Due.Datetime != "" {
				dueDatetime = &t.Due.Datetime
			}
			if t.Due.String != "" {
				dueString = &t.Due.String
			}
			if t.Due.Timezone != "" {
				dueTZ = &t.Due.Timezone
			}
			dueRecurring = boolToInt(t.Due.IsRecurring)
		}
		var deadline *string
		if t.Deadline != nil && t.Deadline.Date != "" {
			deadline = &t.Deadline.Date
		}
		db.ExecContext(ctx,
			`INSERT INTO tasks(
			   id,content,description,project_id,section_id,parent_id,
			   priority,ord,is_completed,
			   due_date,due_datetime,due_string,due_is_recurring,due_timezone,
			   deadline_date,url,comment_count,created_at)
			 VALUES(?,?,?,nullif(?,?),nullif(?,?),nullif(?,?),?,?,?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   content=excluded.content, description=excluded.description,
			   project_id=excluded.project_id, section_id=excluded.section_id,
			   parent_id=excluded.parent_id, priority=excluded.priority,
			   ord=excluded.ord, is_completed=excluded.is_completed,
			   due_date=excluded.due_date, due_datetime=excluded.due_datetime,
			   due_string=excluded.due_string, due_is_recurring=excluded.due_is_recurring,
			   due_timezone=excluded.due_timezone, deadline_date=excluded.deadline_date,
			   url=excluded.url, comment_count=excluded.comment_count`,
			t.ID, t.Content, t.Description,
			t.ProjectID, "", t.SectionID, "", t.ParentID, "",
			t.Priority, t.Order, boolToInt(t.IsCompleted),
			dueDate, dueDatetime, dueString, dueRecurring, dueTZ,
			deadline, t.URL, t.CommentCount, t.CreatedAt)
		db.ExecContext(ctx, `DELETE FROM task_labels WHERE task_id=?`, t.ID)
		for _, label := range t.Labels {
			db.ExecContext(ctx,
				`INSERT OR IGNORE INTO task_labels(task_id,label_name) VALUES(?,?)`,
				t.ID, label)
		}
	}
}

// purgeMissing deletes the rows of table whose id is not in keep — the ones
// the API no longer returns — optionally only where scopeCol = scopeVal.
func purgeMissing(ctx context.Context, tx *sql.Tx, table, scopeCol, scopeVal string, keep []string) error {
	var where []string
	var args []any
	if scopeCol != "" {
		where = append(where, scopeCol+" = ?")
		args = append(args, scopeVal)
	}
	if len(keep) > 0 {
		where = append(where, "id NOT IN ("+strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",")+")")
		for _, id := range keep {
			args = append(args, id)
		}
	}
	q := "DELETE FROM " + table
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("purge %s: %w", table, err)
	}
	return nil
}

func ids[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = id(it)
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullIfEmpty returns nil for an empty string so it stores as SQL NULL rather
// than an empty string, keeping top-level projects distinguishable from a real parent_id.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func init() {
	syncCmd.Flags().StringVarP(&syncProject, "project", "p", "", "sync only this project (name or ID)")
	root.AddCommand(syncCmd)
}
