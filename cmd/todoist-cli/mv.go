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
	mvProject string
	mvSection string
)

var mvCmd = &cobra.Command{
	Use:               "mv <task> [<section>]",
	Short:             "Move a task to a different section or project",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: mvCompleter,
	Long: `Move a task to a different section within the current project, or to a different project.

Within-project (section move):
  mv <task> <section>

Cross-project:
  mv <task> -p <project>
  mv <task> -p <project> -s <section>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		hasProject := cmd.Flags().Changed("project")
		hasSection := cmd.Flags().Changed("section")

		// Validate arg/flag combinations.
		if len(args) == 2 && hasProject {
			return fmt.Errorf("use either a positional section or -p, not both")
		}
		if len(args) == 1 && !hasProject {
			return fmt.Errorf("specify a destination section or use -p to move to a different project")
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

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		// Cross-project move.
		if hasProject {
			projectID, err := tasks.ProjectByName(ctx, conn, mvProject)
			if err != nil {
				return err
			}

			if hasSection {
				sectionID, err := tasks.SectionByName(ctx, conn, mvSection, projectID)
				if err != nil {
					return err
				}
				if err := client.MoveTaskToSection(ctx, task.ID, sectionID); err != nil {
					return err
				}
				conn.ExecContext(ctx,
					`UPDATE tasks SET project_id = ?, section_id = ? WHERE id = ?`,
					projectID, sectionID, task.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "%s → %s / %s\n", task.Content, mvProject, mvSection)
			} else {
				if err := client.MoveTaskToProject(ctx, task.ID, projectID); err != nil {
					return err
				}
				conn.ExecContext(ctx,
					`UPDATE tasks SET project_id = ?, section_id = NULL WHERE id = ?`,
					projectID, task.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", task.Content, mvProject)
			}
			return nil
		}

		// Within-project section move (existing behaviour).
		st, err := loadContext(ctx, conn)
		if err != nil {
			return err
		}
		if !st.HasProject() {
			return fmt.Errorf("no project context — run: td cd <project>")
		}

		sectionID, err := tasks.SectionByName(ctx, conn, args[1], st.ProjectID)
		if err != nil {
			return err
		}

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

	// second positional arg: sections in current project (or target project if -p set)
	projectID := ""
	if p, err := cmd.Flags().GetString("project"); err == nil && p != "" {
		if id, err := tasks.ProjectByName(ctx, conn, p); err == nil {
			projectID = id
		}
	}
	if projectID == "" {
		st, err := loadContext(ctx, conn)
		if err != nil || !st.HasProject() {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		projectID = st.ProjectID
	}

	rows, err := conn.QueryContext(ctx,
		`SELECT name FROM sections WHERE project_id = ? AND is_archived = 0 ORDER BY ord`,
		projectID)
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
	mvCmd.Flags().StringVarP(&mvProject, "project", "p", "", "move to this project")
	mvCmd.Flags().StringVarP(&mvSection, "section", "s", "", "move to this section in the destination project (requires -p)")
	mvCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	mvCmd.RegisterFlagCompletionFunc("section", addSectionCompleter)
	root.AddCommand(mvCmd)
}
