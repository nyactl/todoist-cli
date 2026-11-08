package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:               "done <id>",
	Short:             "Mark a task complete",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: taskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		t, err := tasks.ByID(ctx, conn, args[0])
		if err != nil {
			return err
		}
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		if err := client.CloseTask(ctx, t.ID); err != nil {
			return err
		}
		next := markClosed(ctx, conn, client, *t)
		fmt.Printf("done  %s  %s%s\n", shortID(ctx, t.ID), t.Content, nextDueNote(next))
		return nil
	},
}

// markClosed mirrors a close into the cache and returns the next due date of
// a recurring task. Closing a recurring task only moves its due date, so it is
// fetched again instead of being marked complete (#47).
func markClosed(ctx context.Context, conn *sql.DB, client *todoist.Client, t tasks.Task) *todoist.Due {
	if t.DueRecurring {
		if fresh, err := client.GetTask(ctx, t.ID); err == nil && !fresh.IsCompleted {
			upsertTasks(ctx, conn, []todoist.Task{*fresh})
			return fresh.Due
		}
	}
	conn.ExecContext(ctx, `UPDATE tasks SET is_completed=1 WHERE id=?`, t.ID)
	return nil
}

func nextDueNote(next *todoist.Due) string {
	if next == nil {
		return ""
	}
	return "  (next: " + formatDue(next.Date, next.Datetime) + ")"
}

func init() {
	root.AddCommand(doneCmd)
}
