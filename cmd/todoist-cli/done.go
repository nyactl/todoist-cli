package main

import (
	"fmt"
	"os"

	"todoist-cli/internal/config"
	ryodb "todoist-cli/internal/db"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark a task complete",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		if err := client.CloseTask(cmd.Context(), id); err != nil {
			return err
		}
		if db, err := ryodb.Open(); err == nil {
			defer db.Close()
			db.ExecContext(cmd.Context(),
				`UPDATE tasks SET is_completed=1 WHERE id=?`, id)
		}
		fmt.Fprintf(os.Stderr, "done  %s\n", id[:min(4, len(id))])
		return nil
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	root.AddCommand(doneCmd)
}
