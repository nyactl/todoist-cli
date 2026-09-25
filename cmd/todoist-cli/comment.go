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

var commentLsCmd = &cobra.Command{
	Use:               "ls <task>",
	Short:             "List a task's comments (id, timestamp, first line — tab-separated)",
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

		comments, err := client.GetComments(ctx, task.ID)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		for _, c := range comments {
			// Full comment ID (not shortID) so it can be passed straight to
			// `comment rm`; only the first line keeps the row to one line.
			fmt.Fprintf(out, "%s\t%s\t%s\n", c.ID, formatCommentTime(c.PostedAt), firstLine(c.Content))
		}
		return nil
	},
}

var commentRmCmd = &cobra.Command{
	Use:               "rm <comment-id>",
	Short:             "Delete a comment by its ID (from `comment ls`)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if err := client.DeleteComment(cmd.Context(), args[0]); err != nil {
			return fmt.Errorf("delete comment %q (pass a full comment ID from `todoist-cli comment ls <task>`): %w", args[0], err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
		return nil
	},
}

// firstLine returns the first line of s, so a multi-line comment stays on one
// row in `comment ls`.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func init() {
	commentCmd.AddCommand(commentLsCmd)
	commentCmd.AddCommand(commentRmCmd)
	root.AddCommand(commentCmd)
}
