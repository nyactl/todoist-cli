package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	ryodb "todoist-cli/internal/db"
	"todoist-cli/internal/state"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	lsProject string
	lsLabel   string
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List active tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := ryodb.Open()
		if err != nil {
			return err
		}
		defer db.Close()

		projectID := lsProject
		if projectID == "" {
			st, _ := state.Load()
			if st.HasProject() {
				projectID = st.ProjectID
			}
		}

		return printTasks(cmd.Context(), db, projectID, lsLabel)
	},
}

func printTasks(ctx context.Context, db *sql.DB, projectID, labelFilter string) error {
	q := `SELECT t.id, t.content, t.priority, t.due_date, p.name
	      FROM tasks t
	      LEFT JOIN projects p ON p.id = t.project_id
	      WHERE t.is_completed = 0`
	var args []any

	if projectID != "" {
		q += " AND t.project_id = ?"
		args = append(args, projectID)
	}
	if labelFilter != "" {
		q += " AND EXISTS (SELECT 1 FROM task_labels tl WHERE tl.task_id=t.id AND tl.label_name=?)"
		args = append(args, labelFilter)
	}
	q += " ORDER BY t.priority DESC, t.ord"

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, content, due, proj string
		var priority int
		if err := rows.Scan(&id, &content, &priority, &due, &proj); err != nil {
			return err
		}
		shortID := id
		if len(shortID) > 4 {
			shortID = shortID[:4]
		}
		line := fmt.Sprintf("%s  %s  %s", shortID, priorityMark(priority), content)
		if due != "" {
			line += "  " + due
		}
		if proj != "" {
			line += "  [" + proj + "]"
		}
		fmt.Println(line)
	}
	return rows.Err()
}

func priorityMark(p int) string {
	switch p {
	case 4:
		return "!!"
	case 3:
		return "! "
	case 2:
		return "· "
	default:
		return "  "
	}
}

var addLabelToTask = func(ctx context.Context, client *todoist.Client, taskID string, labels []string) error {
	return nil
}

var _ = strings.Join // keep import

func init() {
	lsCmd.Flags().StringVar(&lsProject, "project", "", "filter by project ID")
	lsCmd.Flags().StringVar(&lsLabel, "label", "", "filter by label name")
	root.AddCommand(lsCmd)
}

func listTasksFromDB(ctx context.Context, db *sql.DB, projectID string) ([]struct{ ID, Content string }, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, content FROM tasks WHERE is_completed=0 AND project_id=? ORDER BY ord`,
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []struct{ ID, Content string }
	for rows.Next() {
		var t struct{ ID, Content string }
		if err := rows.Scan(&t.ID, &t.Content); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
