package main

import (
	"bufio"
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

var commentShowCmd = &cobra.Command{
	Use:               "show <comment-id>...",
	Short:             "Print the full body of one or more comments",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		out := cmd.OutOrStdout()
		for i, id := range args {
			c, err := client.GetComment(ctx, id)
			if err != nil {
				return fmt.Errorf("get comment %q (pass a full comment ID from `todoist-cli comment ls <task>`): %w", id, err)
			}
			if i > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "%s  %s\n", c.ID, formatCommentTime(c.PostedAt))
			fmt.Fprintf(out, "%s\n", c.Content)
		}
		return nil
	},
}

var (
	commentRmAll   bool
	commentRmForce bool
)

var commentRmCmd = &cobra.Command{
	Use:               "rm <comment-id>... | --all <task>",
	Short:             "Delete comments by ID, or every comment on a task with --all",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if commentRmAll {
			return commentRmAllOnTask(cmd, client, args)
		}

		// Delete each ID, echoing what was removed — comments have no undo, so a
		// bare ID in the terminal log is not enough to audit afterwards (#26).
		out := cmd.OutOrStdout()
		var failed int
		for _, id := range args {
			c, err := client.GetComment(ctx, id)
			if err != nil {
				fmt.Fprintf(out, "error: %s: %v\n", id, err)
				failed++
				continue
			}
			if err := client.DeleteComment(ctx, id); err != nil {
				fmt.Fprintf(out, "error: %s: %v\n", id, err)
				failed++
				continue
			}
			fmt.Fprintf(out, "deleted: %s\t%s\t%s\n", id, formatCommentTime(c.PostedAt), firstLine(c.Content))
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d comment(s) could not be deleted", failed, len(args))
		}
		return nil
	},
}

// commentRmAllOnTask deletes every comment on a task, prompting first unless
// forced — the one destructive variant of `comment rm`.
func commentRmAllOnTask(cmd *cobra.Command, client *todoist.Client, args []string) error {
	ctx := cmd.Context()
	if len(args) != 1 {
		return fmt.Errorf("--all takes exactly one task")
	}

	conn, err := db.Open()
	if err != nil {
		return err
	}
	defer conn.Close()

	task, err := tasks.ByID(ctx, conn, args[0])
	if err != nil {
		return err
	}

	comments, err := client.GetComments(ctx, task.ID)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(comments) == 0 {
		fmt.Fprintf(out, "no comments on %s\n", shortID(task.ID))
		return nil
	}

	if !commentRmForce {
		fmt.Fprintf(out, "delete all %d comment(s) on %q? [y/N] ", len(comments), task.Content)
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if !scanner.Scan() {
			return nil
		}
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	var failed int
	for _, c := range comments {
		if err := client.DeleteComment(ctx, c.ID); err != nil {
			fmt.Fprintf(out, "error: %s: %v\n", c.ID, err)
			failed++
			continue
		}
		fmt.Fprintf(out, "deleted: %s\t%s\t%s\n", c.ID, formatCommentTime(c.PostedAt), firstLine(c.Content))
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d comment(s) could not be deleted", failed, len(comments))
	}
	return nil
}

// firstLine returns the first line of s, so a multi-line comment stays on one
// row in `comment ls` and the `comment rm` echo.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func init() {
	commentRmCmd.Flags().BoolVar(&commentRmAll, "all", false, "delete every comment on the given task (prompts unless -f)")
	commentRmCmd.Flags().BoolVarP(&commentRmForce, "force", "f", false, "skip the confirmation prompt for --all")
	commentCmd.AddCommand(commentLsCmd)
	commentCmd.AddCommand(commentShowCmd)
	commentCmd.AddCommand(commentRmCmd)
	root.AddCommand(commentCmd)
}
