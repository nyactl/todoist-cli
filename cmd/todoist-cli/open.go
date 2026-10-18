package main

import (
	"fmt"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:               "open <id>",
	Short:             "Reopen a completed task",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completedTaskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		// Short IDs printed by ls --done resolve through the cache; a full ID
		// of a task that was never cached goes to the API unchanged.
		id := args[0]
		if resolved, ok, err := tasks.IDByPrefix(ctx, conn, id); err != nil {
			return err
		} else if ok {
			id = resolved
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		if err := todoist.New(token).ReopenTask(ctx, id); err != nil {
			return err
		}
		conn.ExecContext(ctx, `UPDATE tasks SET is_completed=0 WHERE id=?`, id)
		fmt.Printf("reopened  %s\n", shortID(ctx, id))
		return nil
	},
}

func completedTaskCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	token, err := config.GetToken()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	ctx := cmd.Context()
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	st, _ := loadContext(ctx, conn)
	client := todoist.New(token)
	now := time.Now()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	res, err := client.GetCompletedSince(ctx, since, st.ProjectID)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	completions := make([]string, len(res.Tasks))
	for i, t := range res.Tasks {
		completions[i] = t.TaskID + "\t" + t.Content
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	root.AddCommand(openCmd)
}
