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

var projectsAddParent string

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
			`INSERT INTO projects(id,name,color,ord,is_archived,is_favorite,view_style)
			 VALUES(?,?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name, color=excluded.color, ord=excluded.ord,
			   is_archived=excluded.is_archived, is_favorite=excluded.is_favorite,
			   view_style=excluded.view_style`,
			proj.ID, proj.Name, proj.Color, proj.Order,
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
	projectsCmd.AddCommand(projectsAddCmd)
	projectsCmd.AddCommand(projectsRmCmd)
	root.AddCommand(projectsCmd)
}
