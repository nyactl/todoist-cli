package main

import (
	"fmt"

	"todoist-cli/internal/config"
	"todoist-cli/internal/db"
	"todoist-cli/internal/tasks"
	"todoist-cli/internal/todoist"

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
		conn.ExecContext(ctx, `UPDATE tasks SET is_completed=1 WHERE id=?`, t.ID)
		fmt.Printf("done  %s  %s\n", shortID(t.ID), t.Content)
		return nil
	},
}

func init() {
	root.AddCommand(doneCmd)
}
