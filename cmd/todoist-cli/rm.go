package main

import (
	"fmt"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:               "rm <task>",
	Short:             "Delete a task",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: taskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		task, err := tasks.ByID(ctx, conn, args[0])
		if err != nil {
			return err
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if err := client.DeleteTask(ctx, task.ID); err != nil {
			return err
		}

		if _, err := conn.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, task.ID); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", task.Content)
		return nil
	},
}

func init() {
	root.AddCommand(rmCmd)
}
