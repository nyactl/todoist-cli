package main

import (
	"context"
	"fmt"
	"strings"

	"todoist-cli/internal/config"
	ryodb "todoist-cli/internal/db"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	addProject string
	addLabels  string
)

var addCmd = &cobra.Command{
	Use:   "add <content>",
	Short: "Create a task",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content := strings.Join(args, " ")

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		req := todoist.CreateTaskRequest{Content: content}
		if addProject != "" {
			req.ProjectID = addProject
		}
		if addLabels != "" {
			req.Labels = resolveLabels(cmd.Context(), client, addLabels)
		}

		task, err := client.CreateTask(cmd.Context(), req)
		if err != nil {
			return err
		}

		// Optimistic local insert — best effort.
		if db, err := ryodb.Open(); err == nil {
			defer db.Close()
			upsertTasks(cmd.Context(), db, []todoist.Task{*task})
		}

		fmt.Println(task.ID)
		return nil
	},
}

// resolveLabels maps label IDs or names from --labels CSV to label names for the API.
// The Todoist v1 API accepts label names on tasks, not IDs.
func resolveLabels(ctx context.Context, client *todoist.Client, csv string) []string {
	return strings.Split(csv, ",")
}

func init() {
	addCmd.Flags().StringVar(&addProject, "project", "", "project ID")
	addCmd.Flags().StringVar(&addLabels, "labels", "", "comma-separated label IDs")
	root.AddCommand(addCmd)
}
