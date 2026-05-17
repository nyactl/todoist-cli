package main

import (
	"fmt"

	"github.com/nyactl/todoist-cli/internal/db"

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

func init() {
	root.AddCommand(sectionsCmd)
}
