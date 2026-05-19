package main

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"

	"github.com/spf13/cobra"
)

func taskURL(id string) string {
	return "https://app.todoist.com/app/task/" + id
}

var cpCmd = &cobra.Command{
	Use:               "cp <task>",
	Short:             "Copy task URL to clipboard",
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

		url := taskURL(task.ID)

		if err := clipboard.WriteAll(url); err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "copied  %s\n", url)
		return nil
	},
}

func init() {
	root.AddCommand(cpCmd)
}
