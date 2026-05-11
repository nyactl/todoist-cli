package main

import (
	"context"
	"database/sql"
	"fmt"

	ryodb "todoist-cli/internal/db"

	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all projects (id<TAB>name per line)",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := ryodb.Open()
		if err != nil {
			return err
		}
		defer db.Close()
		return printProjects(cmd.Context(), db)
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
	root.AddCommand(projectsCmd)
}
