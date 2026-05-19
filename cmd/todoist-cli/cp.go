package main

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

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

		taskURL := task.URL
		if taskURL == "" {
			// URL missing from cache — fetch live from API
			token, err := config.GetToken()
			if err != nil {
				return fmt.Errorf("task has no URL in local cache: %w", err)
			}
			live, err := todoist.New(token).GetTask(ctx, task.ID)
			if err != nil {
				return fmt.Errorf("task has no URL in local cache and live fetch failed: %w", err)
			}
			taskURL = live.URL
			if taskURL == "" {
				return fmt.Errorf("task has no URL")
			}
		}

		if err := clipboard.WriteAll(taskURL); err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "copied  %s\n", taskURL)
		return nil
	},
}

func init() {
	root.AddCommand(cpCmd)
}
