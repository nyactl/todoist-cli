package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

		type step struct {
			name string
			fn   func(context.Context, *sql.DB, *todoist.Client) (int, error)
		}
		for _, s := range []step{
			{"labels", syncLabels},
			{"projects", syncProjects},
			{"sections", syncSections},
			{"tasks", syncTasks},
		} {
			n, err := s.fn(ctx, conn, client)
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

func syncLabels(ctx context.Context, db *sql.DB, client *todoist.Client) (int, error) {
	items, err := client.GetLabels(ctx)
	if err != nil {
		return 0, err
	}
	for _, l := range items {
		_, err := db.ExecContext(ctx,
			`INSERT INTO labels(id,name,color,ord,is_favorite) VALUES(?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color,
			   ord=excluded.ord, is_favorite=excluded.is_favorite`,
			l.ID, l.Name, l.Color, l.Order, boolToInt(l.IsFavorite))
		if err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func syncProjects(ctx context.Context, db *sql.DB, client *todoist.Client) (int, error) {
	items, err := client.GetProjects(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range items {
		_, err := db.ExecContext(ctx,
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
	return len(items), nil
}

func syncSections(ctx context.Context, db *sql.DB, client *todoist.Client) (int, error) {
	items, err := client.GetSections(ctx)
	if err != nil {
		return 0, err
	}
	for i, s := range items {
		_, err := db.ExecContext(ctx,
			`INSERT INTO sections(id,name,project_id,ord,is_archived) VALUES(?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, project_id=excluded.project_id,
			   ord=excluded.ord, is_archived=excluded.is_archived`,
			s.ID, s.Name, s.ProjectID, i, boolToInt(s.IsArchived))
		if err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func syncTasks(ctx context.Context, db *sql.DB, client *todoist.Client) (int, error) {
	items, err := client.GetTasks(ctx, "")
	if err != nil {
		return 0, err
	}
	ids := make([]any, len(items))
	for i, t := range items {
		ids[i] = t.ID
	}
	if len(ids) > 0 {
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		if _, err := db.ExecContext(ctx,
			`DELETE FROM tasks WHERE id NOT IN (`+placeholders+`)`, ids...); err != nil {
			return 0, fmt.Errorf("purge deleted tasks: %w", err)
		}
	} else {
		if _, err := db.ExecContext(ctx, `DELETE FROM tasks`); err != nil {
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
		_, err := db.ExecContext(ctx,
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
		if _, err := db.ExecContext(ctx,
			`DELETE FROM task_labels WHERE task_id=?`, t.ID); err != nil {
			return 0, err
		}
		for _, label := range t.Labels {
			if _, err := db.ExecContext(ctx,
				`INSERT OR IGNORE INTO task_labels(task_id,label_name) VALUES(?,?)`,
				t.ID, label); err != nil {
				return 0, err
			}
		}
	}
	return len(items), nil
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
