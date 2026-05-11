package main

import (
	"context"
	"database/sql"
	"fmt"

	"todoist-cli/internal/db"

	"github.com/spf13/cobra"
)

var labelsCmd = &cobra.Command{
	Use:               "labels",
	Short:             "List all personal labels (id and name, tab-separated)",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()
		return printLabels(cmd.Context(), conn)
	},
}

func printLabels(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name FROM labels ORDER BY ord`)
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
	root.AddCommand(labelsCmd)
}
