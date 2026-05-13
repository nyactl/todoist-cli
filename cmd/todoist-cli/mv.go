package main

import (
	"fmt"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:               "mv <task> <section>",
	Short:             "Move a task to a different section within the current project",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: mvCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		st, err := loadContext(ctx, conn)
		if err != nil {
			return err
		}
		if !st.HasProject() {
			return fmt.Errorf("no project context — run: td cd <project>")
		}

		task, err := tasks.ByID(ctx, conn, args[0])
		if err != nil {
			return err
		}

		sectionID, err := tasks.SectionByName(ctx, conn, args[1], st.ProjectID)
		if err != nil {
			return err
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if err := client.MoveTaskToSection(ctx, task.ID, sectionID); err != nil {
			return err
		}

		if _, err := conn.ExecContext(ctx,
			`UPDATE tasks SET section_id = ? WHERE id = ?`, sectionID, task.ID); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", task.Content, args[1])
		return nil
	},
}

func mvCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	ctx := cmd.Context()

	if len(args) == 0 {
		return taskCompleter(cmd, args, toComplete)
	}

	// second arg: sections in current project
	st, err := loadContext(ctx, conn)
	if err != nil || !st.HasProject() {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rows, err := conn.QueryContext(ctx,
		`SELECT name FROM sections WHERE project_id = ? AND is_archived = 0 ORDER BY ord`,
		st.ProjectID)
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

func init() {
	root.AddCommand(mvCmd)
}
