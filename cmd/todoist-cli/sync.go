package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

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

		// Fetch all resource types concurrently — each is independent.
		type fetchResult struct {
			labels   []todoist.Label
			projects []todoist.Project
			sections []todoist.Section
			tasks    []todoist.Task
		}
		var (
			fetched fetchResult
			mu      sync.Mutex
			wg      sync.WaitGroup
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
		go func() { defer wg.Done(); items, err := client.GetLabels(ctx); if err != nil { setErr(fmt.Errorf("labels: %w", err)); return }; mu.Lock(); fetched.labels = items; mu.Unlock() }()
		go func() { defer wg.Done(); items, err := client.GetProjects(ctx); if err != nil { setErr(fmt.Errorf("projects: %w", err)); return }; mu.Lock(); fetched.projects = items; mu.Unlock() }()
		go func() { defer wg.Done(); items, err := client.GetSections(ctx); if err != nil { setErr(fmt.Errorf("sections: %w", err)); return }; mu.Lock(); fetched.sections = items; mu.Unlock() }()
		go func() { defer wg.Done(); items, err := client.GetTasks(ctx, ""); if err != nil { setErr(fmt.Errorf("tasks: %w", err)); return }; mu.Lock(); fetched.tasks = items; mu.Unlock() }()
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
			{"sections", func(ctx context.Context, db *sql.DB) (int, error) { return writeSections(ctx, db, fetched.sections) }},
			{"tasks", func(ctx context.Context, db *sql.DB) (int, error) { return writeTasks(ctx, db, fetched.tasks) }},
		} {
			n, err := s.fn(ctx, conn)
			if err != nil {
				return fmt.Errorf("sync %s: %w", s.name, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %d\n", s.name, n)
		}

		_, err = conn.ExecContext(ctx,
			`INSERT INTO sync_state(key,value) VALUES('last_synced_at',?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			time.Now().UTC().Format(time.RFC3339))
		return err
	},
}

func writeLabels(ctx context.Context, db *sql.DB, items []todoist.Label) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
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
	for _, p := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO projects(id,name,color,ord,is_archived,is_favorite,view_style)
			 VALUES(?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, ord=excluded.ord,
			   is_archived=excluded.is_archived, is_favorite=excluded.is_favorite,
			   view_style=excluded.view_style`,
			p.ID, p.Name, p.Color, p.Order,
			boolToInt(p.IsArchived), boolToInt(p.IsFavorite), p.ViewStyle)
		if err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit()
}

func writeSections(ctx context.Context, db *sql.DB, items []todoist.Section) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
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

func writeTasks(ctx context.Context, db *sql.DB, items []todoist.Task) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	ids := make([]any, len(items))
	for i, t := range items {
		ids[i] = t.ID
	}
	if len(ids) > 0 {
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM tasks WHERE id NOT IN (`+placeholders+`)`, ids...); err != nil {
			return 0, fmt.Errorf("purge deleted tasks: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM tasks`); err != nil {
			return 0, fmt.Errorf("purge all tasks: %w", err)
		}
	}
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
		_, err := tx.ExecContext(ctx,
			`INSERT INTO tasks(
			   id,content,description,project_id,section_id,parent_id,
			   priority,ord,is_completed,
			   due_date,due_datetime,due_string,due_is_recurring,due_timezone,
			   url,comment_count,created_at)
			 VALUES(?,?,?,nullif(?,?),nullif(?,?),nullif(?,?),?,?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   content=excluded.content, description=excluded.description,
			   project_id=excluded.project_id, section_id=excluded.section_id,
			   parent_id=excluded.parent_id, priority=excluded.priority,
			   ord=excluded.ord, is_completed=excluded.is_completed,
			   due_date=excluded.due_date, due_datetime=excluded.due_datetime,
			   due_string=excluded.due_string, due_is_recurring=excluded.due_is_recurring,
			   due_timezone=excluded.due_timezone,
			   url=excluded.url, comment_count=excluded.comment_count`,
			t.ID, t.Content, t.Description,
			t.ProjectID, "", t.SectionID, "", t.ParentID, "",
			t.Priority, t.Order, boolToInt(t.IsCompleted),
			dueDate, dueDatetime, dueString, dueRecurring, dueTZ,
			t.URL, t.CommentCount, t.CreatedAt)
		if err != nil {
			return 0, fmt.Errorf("task %s: %w", t.ID, err)
		}
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
	return len(items), tx.Commit()
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
		db.ExecContext(ctx,
			`INSERT INTO tasks(
			   id,content,description,project_id,section_id,parent_id,
			   priority,ord,is_completed,
			   due_date,due_datetime,due_string,due_is_recurring,due_timezone,
			   url,comment_count,created_at)
			 VALUES(?,?,?,nullif(?,?),nullif(?,?),nullif(?,?),?,?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   content=excluded.content, description=excluded.description,
			   project_id=excluded.project_id, section_id=excluded.section_id,
			   parent_id=excluded.parent_id, priority=excluded.priority,
			   ord=excluded.ord, is_completed=excluded.is_completed,
			   due_date=excluded.due_date, due_datetime=excluded.due_datetime,
			   due_string=excluded.due_string, due_is_recurring=excluded.due_is_recurring,
			   due_timezone=excluded.due_timezone,
			   url=excluded.url, comment_count=excluded.comment_count`,
			t.ID, t.Content, t.Description,
			t.ProjectID, "", t.SectionID, "", t.ParentID, "",
			t.Priority, t.Order, boolToInt(t.IsCompleted),
			dueDate, dueDatetime, dueString, dueRecurring, dueTZ,
			t.URL, t.CommentCount, t.CreatedAt)
		db.ExecContext(ctx, `DELETE FROM task_labels WHERE task_id=?`, t.ID)
		for _, label := range t.Labels {
			db.ExecContext(ctx,
				`INSERT OR IGNORE INTO task_labels(task_id,label_name) VALUES(?,?)`,
				t.ID, label)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func init() {
	root.AddCommand(syncCmd)
}
