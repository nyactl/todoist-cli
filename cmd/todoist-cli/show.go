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

var showCmd = &cobra.Command{
	Use:               "show <id>",
	Short:             "Show full task details (live API call)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: taskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// resolve prefix → full ID from local cache if possible
		taskID := args[0]
		if conn, err := db.Open(); err == nil {
			if t, err := tasks.ByID(ctx, conn, args[0]); err == nil {
				taskID = t.ID
			}
			conn.Close()
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		t, err := client.GetTask(ctx, taskID)
		if err != nil {
			return err
		}

		fmt.Printf("%s  %s\n", shortID(t.ID), t.Content)
		if t.Description != "" {
			fmt.Printf("\n%s\n", t.Description)
		}
		if t.Due != nil {
			due := t.Due.Date
			if t.Due.Datetime != "" {
				due = t.Due.Datetime
			}
			fmt.Printf("\ndue  %s", due)
			if t.Due.String != "" {
				fmt.Printf("  (%s)", t.Due.String)
			}
			fmt.Println()
		}
		if len(t.Labels) > 0 {
			fmt.Printf("labels  %s\n", strings.Join(t.Labels, ", "))
		}

		// subtasks from local cache
		if conn, err := db.Open(); err == nil {
			if subs, err := tasks.Subtasks(ctx, conn, t.ID); err == nil && len(subs) > 0 {
				fmt.Println("\nsubtasks")
				for _, s := range subs {
					fmt.Printf("  %s  %s\n", shortID(s.ID), s.Content)
				}
			}
			conn.Close()
		}

		// comments from API
		comments, err := client.GetComments(ctx, t.ID)
		if err == nil && len(comments) > 0 {
			fmt.Println("\ncomments")
			for _, c := range comments {
				fmt.Printf("  %s  %s\n", c.PostedAt[:10], c.Content)
			}
		}

		if t.URL != "" {
			fmt.Printf("\n%s\n", t.URL)
		}
		return nil
	},
}

var showProject string

func init() {
	showCmd.Flags().StringVarP(&showProject, "project", "p", "", "filter task completion by project name")
	showCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	root.AddCommand(showCmd)
}
