package main

import (
	"fmt"
	"strings"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:               "comment <task> <text>",
	Short:             "Add a comment to a task",
	Args:              cobra.MinimumNArgs(2),
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

		content := strings.Join(args[1:], " ")
		if strings.TrimSpace(content) == "" {
			return fmt.Errorf("comment text cannot be empty")
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		comment, err := client.PostComment(ctx, task.ID, content)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "commented  %s  %s\n", shortID(task.ID), comment.Content)
		return nil
	},
}

func init() {
	root.AddCommand(commentCmd)
}
