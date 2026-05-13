package main

import (
	"fmt"
	"os"

	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/state"
	"github.com/nyactl/todoist-cli/internal/tasks"

	"github.com/spf13/cobra"
)

var cdCmd = &cobra.Command{
	Use:               "cd [project-name]",
	Short:             "Set active project context (no args clears it)",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: projectCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if err := state.Clear(); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintln(os.Stderr, "context cleared")
			return nil
		}

		name := args[0]
		ctx := cmd.Context()
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		id, err := tasks.ProjectByName(ctx, conn, name)
		if err != nil {
			return err
		}
		if err := state.Save(&state.State{ProjectID: id, ProjectName: name}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "→ %s\n", name)
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print active project as id and name (tab-separated), empty if none",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.Load()
		if err != nil || !st.HasProject() {
			return nil // empty stdout — not an error
		}
		fmt.Printf("%s\t%s\n", st.ProjectID, st.ProjectName)
		return nil
	},
}

func init() {
	root.AddCommand(cdCmd)
	root.AddCommand(contextCmd)
}
