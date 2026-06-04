package main

import (
	"fmt"
	"strings"

	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:               "search <query>",
	Short:             "Search tasks by content or description across all projects",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.Join(args, " ")
		ctx := cmd.Context()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		ts, err := tasks.Search(ctx, conn, q)
		if err != nil {
			return err
		}
		if len(ts) == 0 {
			fmt.Println("no results")
			return nil
		}
		printByProject(ts)
		return nil
	},
}

func init() {
	root.AddCommand(searchCmd)
}
