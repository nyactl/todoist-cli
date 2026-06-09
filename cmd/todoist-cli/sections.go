package main

import (
	"fmt"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var sectionsCmd = &cobra.Command{
	Use:               "sections",
	Short:             "List sections in the active project (id and name, tab-separated)",
	ValidArgsFunction: cobra.NoFileCompletions,
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

		rows, err := conn.QueryContext(ctx,
			`SELECT id, name FROM sections WHERE project_id = ? AND is_archived = 0 ORDER BY ord`,
			st.ProjectID)
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
	},
}

var sectionsRmCmd = &cobra.Command{
	Use:               "rm <section>",
	Short:             "Delete a section from the active project",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: addSectionCompleter,
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

		sectionID, err := tasks.SectionByName(ctx, conn, args[0], st.ProjectID)
		if err != nil {
			return err
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}

		client := todoist.New(token)
		if err := client.DeleteSection(ctx, sectionID); err != nil {
			return err
		}

		conn.ExecContext(ctx, `DELETE FROM sections WHERE id = ?`, sectionID)

		fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
		return nil
	},
}

func init() {
	sectionsCmd.AddCommand(sectionsRmCmd)
	root.AddCommand(sectionsCmd)
}
