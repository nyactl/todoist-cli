package main

import (
	"fmt"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
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
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		if err := client.ReopenTask(ctx, args[0]); err != nil {
			return err
		}
		conn, err := db.Open()
		if err != nil {
			return nil
		}
		defer conn.Close()
		conn.ExecContext(ctx, `UPDATE tasks SET is_completed=0 WHERE id=?`, args[0])
		fmt.Printf("reopened  %s\n", shortID(args[0]))
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
