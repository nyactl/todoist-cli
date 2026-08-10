package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	projectsAddParent string
	projectsRmForce   bool
)

var projectsCmd = &cobra.Command{
	Use:               "projects",
	Short:             "List all projects (id and name, tab-separated)",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()
		return printProjects(cmd.Context(), conn)
	},
}

var projectsAddCmd = &cobra.Command{
	Use:               "add <name>",
	Short:             "Create a project",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, " ")
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

		req := todoist.CreateProjectRequest{Name: name}
		if projectsAddParent != "" {
			parentID, err := tasks.ProjectByName(ctx, conn, projectsAddParent)
			if err != nil {
				return err
			}
			req.ParentID = parentID
		}

		proj, err := client.CreateProject(ctx, req)
		if err != nil {
			return err
		}

		conn.ExecContext(ctx,
			`INSERT INTO projects(id,name,color,ord,parent_id,is_archived,is_favorite,view_style)
			 VALUES(?,?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, ord=excluded.ord,
			   parent_id=excluded.parent_id,
			   is_archived=excluded.is_archived, is_favorite=excluded.is_favorite,
			   view_style=excluded.view_style`,
			proj.ID, proj.Name, proj.Color, proj.Order, nullIfEmpty(proj.ParentID),
			boolToInt(proj.IsArchived), boolToInt(proj.IsFavorite), proj.ViewStyle)

		fmt.Println(proj.ID)
		return nil
	},
}

var projectsRmCmd = &cobra.Command{
	Use:               "rm <project>",
	Short:             "Delete a project",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: projectCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		projectID, err := tasks.ProjectByName(ctx, conn, args[0])
		if err != nil {
			return err
		}

		if !projectsRmForce {
			// Deleting a project cascades on Todoist's side: tasks in any nested
			// sub-project are permanently deleted too. Count the whole subtree,
			// not just direct tasks, so an "empty-looking" parent can't silently
			// wipe out its sub-projects' tasks.
			var subprojects, directTasks, subtreeTasks int
			err := conn.QueryRowContext(ctx, `
				WITH RECURSIVE subtree(id) AS (
					SELECT id FROM projects WHERE id = ?
					UNION ALL
					SELECT p.id FROM projects p JOIN subtree s ON p.parent_id = s.id
				)
				SELECT
					(SELECT COUNT(*) FROM projects WHERE parent_id = ?),
					(SELECT COUNT(*) FROM tasks WHERE project_id = ?),
					(SELECT COUNT(*) FROM tasks WHERE project_id IN (SELECT id FROM subtree))`,
				projectID, projectID, projectID).Scan(&subprojects, &directTasks, &subtreeTasks)
			if err != nil {
				return err
			}
			if subtreeTasks > 0 {
				if subprojects > 0 {
					return fmt.Errorf("project %q is not empty (%d tasks total; %d in %d sub-project(s) that Todoist would permanently delete) — use --force to delete anyway",
						args[0], subtreeTasks, subtreeTasks-directTasks, subprojects)
				}
				return fmt.Errorf("project %q is not empty (%d tasks) — use --force to delete anyway", args[0], subtreeTasks)
			}
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if err := client.DeleteProject(ctx, projectID); err != nil {
			return err
		}

		conn.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, projectID)

		fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
		return nil
	},
}

var projectsMvCmd = &cobra.Command{
	Use:               "mv <project> <new-name>",
	Short:             "Rename a project",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: projectCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		newName := strings.Join(args[1:], " ")

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		projectID, err := tasks.ProjectByName(ctx, conn, args[0])
		if err != nil {
			return err
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		proj, err := client.UpdateProject(ctx, projectID, todoist.UpdateProjectRequest{Name: newName})
		if err != nil {
			return err
		}

		conn.ExecContext(ctx, `UPDATE projects SET name = ? WHERE id = ?`, proj.Name, projectID)

		fmt.Fprintf(cmd.OutOrStdout(), "renamed: %s -> %s\n", args[0], proj.Name)
		return nil
	},
}

func printProjects(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name FROM projects WHERE is_archived=0 ORDER BY ord`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		fmt.Printf("%s\t%s\n", id, name)
	}
	return rows.Err()
}

func init() {
	projectsAddCmd.Flags().StringVar(&projectsAddParent, "parent", "", "parent project name — creates a sub-project")
	projectsAddCmd.RegisterFlagCompletionFunc("parent", projectCompleter)
	projectsRmCmd.Flags().BoolVarP(&projectsRmForce, "force", "f", false, "delete even if the project still has tasks")
	projectsCmd.AddCommand(projectsAddCmd)
	projectsCmd.AddCommand(projectsRmCmd)
	projectsCmd.AddCommand(projectsMvCmd)
	root.AddCommand(projectsCmd)
}
