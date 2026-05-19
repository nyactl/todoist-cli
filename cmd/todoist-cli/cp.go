package main

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"

	"github.com/spf13/cobra"
)

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

		if task.URL == "" {
			return fmt.Errorf("task has no URL — run: todoist-cli sync")
		}

		if err := clipboard.WriteAll(task.URL); err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "copied  %s\n", task.URL)
		return nil
	},
}

func init() {
	root.AddCommand(cpCmd)
}
