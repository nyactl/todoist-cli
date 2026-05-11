package main

import (
	"context"
	"database/sql"
	"fmt"

	"todoist-cli/internal/config"
	ryodb "todoist-cli/internal/db"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull projects, labels, and tasks from Todoist into local cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		db, err := ryodb.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		ctx := cmd.Context()

		projects, err := client.GetProjects(ctx)
		if err != nil {
			return fmt.Errorf("fetch projects: %w", err)
		}
		if err := upsertProjects(ctx, db, projects); err != nil {
			return err
		}

		labels, err := client.GetLabels(ctx)
		if err != nil {
			return fmt.Errorf("fetch labels: %w", err)
		}
		if err := upsertLabels(ctx, db, labels); err != nil {
			return err
		}

		tasks, err := client.GetTasks(ctx, "")
		if err != nil {
			return fmt.Errorf("fetch tasks: %w", err)
		}
		if err := upsertTasks(ctx, db, tasks); err != nil {
			return err
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "synced %d projects, %d labels, %d tasks\n",
			len(projects), len(labels), len(tasks))
		return nil
	},
}

func upsertProjects(ctx context.Context, db *sql.DB, projects []todoist.Project) error {
	for _, p := range projects {
		_, err := db.ExecContext(ctx,
			`INSERT INTO projects(id,name,color,parent_id,ord,is_archived,is_favorite)
			 VALUES(?,?,?,nullif(?,?),?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, parent_id=excluded.parent_id,
			   ord=excluded.ord, is_archived=excluded.is_archived, is_favorite=excluded.is_favorite`,
			p.ID, p.Name, p.Color, p.ParentID, "", p.Order,
			boolInt(p.IsArchived), boolInt(p.IsFavorite))
		if err != nil {
			return fmt.Errorf("upsert project %s: %w", p.ID, err)
		}
	}
	return nil
}

func upsertLabels(ctx context.Context, db *sql.DB, labels []todoist.Label) error {
	for _, l := range labels {
		_, err := db.ExecContext(ctx,
			`INSERT INTO labels(id,name,color,ord,is_favorite)
			 VALUES(?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, ord=excluded.ord, is_favorite=excluded.is_favorite`,
			l.ID, l.Name, l.Color, l.Order, boolInt(l.IsFavorite))
		if err != nil {
			return fmt.Errorf("upsert label %s: %w", l.ID, err)
		}
	}
	return nil
}

func upsertTasks(ctx context.Context, db *sql.DB, tasks []todoist.Task) error {
	for _, t := range tasks {
		dueDate, dueDatetime, dueString, dueRecurring := parseDue(t.Due)
		_, err := db.ExecContext(ctx,
			`INSERT INTO tasks(id,content,description,project_id,section_id,parent_id,
			   priority,ord,is_completed,due_date,due_datetime,due_string,due_is_recurring,url,created_at)
			 VALUES(?,?,?,nullif(?,?),nullif(?,?),nullif(?,?),?,?,0,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   content=excluded.content, project_id=excluded.project_id,
			   section_id=excluded.section_id, priority=excluded.priority,
			   ord=excluded.ord, due_date=excluded.due_date, due_datetime=excluded.due_datetime,
			   due_string=excluded.due_string, due_is_recurring=excluded.due_is_recurring`,
			t.ID, t.Content, t.Description,
			t.ProjectID, "", t.SectionID, "", t.ParentID, "",
			t.Priority, t.Order,
			dueDate, dueDatetime, dueString, boolInt(dueRecurring),
			t.URL, t.CreatedAt)
		if err != nil {
			return fmt.Errorf("upsert task %s: %w", t.ID, err)
		}
		for _, lbl := range t.Labels {
			_, _ = db.ExecContext(ctx,
				`INSERT OR IGNORE INTO task_labels(task_id,label_name) VALUES(?,?)`,
				t.ID, lbl)
		}
	}
	return nil
}

func parseDue(due *todoist.Due) (date, datetime, str string, recurring bool) {
	if due == nil {
		return "", "", "", false
	}
	return due.Date, due.Datetime, due.String, due.IsRecurring
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func init() {
	root.AddCommand(syncCmd)
}
