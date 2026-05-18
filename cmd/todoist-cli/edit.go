package main

import (
	"fmt"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	editContent     string
	editDue         string
	editPriority    int
	editDescription string
	editLabels      []string
	editProject     string
)

var editCmd = &cobra.Command{
	Use:               "edit <task>",
	Short:             "Edit task properties (only provided flags are updated)",
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

		// Build partial update — only include explicitly provided flags.
		// Use map[string]any so that due_string: "" serializes correctly
		// (struct omitempty would drop empty strings).
		fields := map[string]any{}
		if cmd.Flags().Changed("content") {
			fields["content"] = editContent
		}
		if cmd.Flags().Changed("description") {
			fields["description"] = editDescription
		}
		if cmd.Flags().Changed("priority") {
			if editPriority < 1 || editPriority > 4 {
				return fmt.Errorf("priority must be between 1 and 4")
			}
			fields["priority"] = editPriority
		}
		if cmd.Flags().Changed("label") {
			fields["labels"] = editLabels
		}
		if cmd.Flags().Changed("due") {
			fields["due_string"] = editDue // empty string clears the due date
		}

		var updated *todoist.Task
		if len(fields) > 0 {
			updated, err = client.UpdateTaskFields(ctx, task.ID, fields)
			if err != nil {
				return err
			}
			upsertTasks(ctx, conn, []todoist.Task{*updated})
		}

		if cmd.Flags().Changed("project") {
			projectID, err := tasks.ProjectByName(ctx, conn, editProject)
			if err != nil {
				return err
			}
			if err := client.MoveTaskToProject(ctx, task.ID, projectID); err != nil {
				return err
			}
			conn.ExecContext(ctx,
				`UPDATE tasks SET project_id = ?, section_id = NULL WHERE id = ?`,
				projectID, task.ID)
		}

		if updated == nil && !cmd.Flags().Changed("project") {
			return fmt.Errorf("no changes — provide at least one flag")
		}

		// Print summary using the latest known content
		content := task.Content
		if updated != nil {
			content = updated.Content
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated  %s  %s\n", shortID(task.ID), content)
		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&editContent, "content", "c", "", "replace task title")
	editCmd.Flags().StringVarP(&editDue, "due", "D", "", "due date in natural language; empty string clears it")
	editCmd.Flags().IntVarP(&editPriority, "priority", "P", 0, "priority 1–4 (1=normal, 4=urgent)")
	editCmd.Flags().StringVarP(&editDescription, "description", "d", "", "replace description")
	editCmd.Flags().StringArrayVarP(&editLabels, "label", "l", nil, "replace labels (repeatable: -l urgent -l work)")
	editCmd.Flags().StringVarP(&editProject, "project", "p", "", "move task to a different project")
	editCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	editCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	root.AddCommand(editCmd)
}
