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

var (
	addProject     string
	addSection     string
	addLabels      []string
	addDescription string
	addDue         string
)

var addCmd = &cobra.Command{
	Use:               "add <content>",
	Short:             "Create a task",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		content := strings.Join(args, " ")
		ctx := cmd.Context()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		req := todoist.CreateTaskRequest{Content: content, Description: addDescription, DueString: addDue}
		if addProject != "" {
			id, err := tasks.ProjectByName(ctx, conn, addProject)
			if err != nil {
				return err
			}
			req.ProjectID = id
		} else {
			st, err := loadContext(ctx, conn)
			if err != nil {
				return err
			}
			if st.HasProject() {
				req.ProjectID = st.ProjectID
			}
		}
		if addSection != "" {
			if req.ProjectID == "" {
				return fmt.Errorf("--section requires a project context — run: td cd <project> or use --project")
			}
			sectionID, err := tasks.SectionByName(ctx, conn, addSection, req.ProjectID)
			if err != nil {
				return err
			}
			req.SectionID = sectionID
		}
		if len(addLabels) > 0 {
			req.Labels = addLabels
		}

		task, err := client.CreateTask(ctx, req)
		if err != nil {
			return err
		}

		upsertTasks(ctx, conn, []todoist.Task{*task})

		fmt.Println(task.ID)
		return nil
	},
}

func addSectionCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	ctx := cmd.Context()

	var projectID string
	if p, err := cmd.Flags().GetString("project"); err == nil && p != "" {
		if id, err := tasks.ProjectByName(ctx, conn, p); err == nil {
			projectID = id
		}
	}
	if projectID == "" {
		if st, _ := loadContext(ctx, conn); st.HasProject() {
			projectID = st.ProjectID
		}
	}
	if projectID == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rows, err := conn.QueryContext(ctx,
		`SELECT name FROM sections WHERE project_id = ? AND is_archived = 0 ORDER BY ord`, projectID)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out = append(out, name)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func projectCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	rows, err := conn.QueryContext(cmd.Context(),
		`SELECT id, name FROM projects WHERE is_archived=0 ORDER BY ord`)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out = append(out, name+"\t"+id)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func labelCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	rows, err := conn.QueryContext(cmd.Context(),
		`SELECT id, name FROM labels ORDER BY ord`)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out = append(out, name+"\t"+id)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	addCmd.Flags().StringVarP(&addProject, "project", "p", "", "project name")
	addCmd.Flags().StringVarP(&addSection, "section", "s", "", "section name (requires project context)")
	addCmd.Flags().StringArrayVarP(&addLabels, "label", "l", nil, "label name (repeatable: -l <name> -l <name>)")
	addCmd.Flags().StringVarP(&addDescription, "description", "d", "", "task description")
	addCmd.Flags().StringVarP(&addDue, "due", "D", "", "due date in natural language (e.g. \"tomorrow\", \"every monday\")")
	addCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	addCmd.RegisterFlagCompletionFunc("section", addSectionCompleter)
	addCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	root.AddCommand(addCmd)
}
