package main

import (
	"fmt"
	"os"

	"todoist-cli/internal/state"

	"github.com/spf13/cobra"
)

var cdCmd = &cobra.Command{
	Use:   "cd [project-name]",
	Short: "Set active project context (no args clears it)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if err := state.Clear(); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintln(os.Stderr, "context cleared")
			return nil
		}

		name := args[0]
		// TODO: resolve name → ID from local cache and verify it exists.
		// For now store as-is; sync will validate.
		if err := state.Save(&state.State{ProjectName: name}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "→ %s\n", name)
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print active project (id<TAB>name), empty if none — used by ryo",
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
