package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"

	"github.com/spf13/cobra"
)

var pickLabels []string

var pickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Interactively select a task with fzf — prints task ID for composition",
	Example: `  td done $(td pick)
  td show $(td pick)
  td cp $(td pick)
  td edit $(td pick) -D tomorrow`,
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("fzf"); err != nil {
			return fmt.Errorf("fzf not found — install it from https://github.com/junegunn/fzf")
		}

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx := cmd.Context()
		st, err := loadContext(ctx, conn)
		if err != nil {
			return err
		}

		var ts []tasks.Task
		if len(pickLabels) > 0 {
			projectID := ""
			if st.HasProject() {
				projectID = st.ProjectID
			}
			ts, err = tasks.ByLabels(ctx, conn, pickLabels, projectID)
		} else if st.HasProject() {
			ts, err = tasks.ByProject(ctx, conn, st.ProjectID)
		} else {
			ts, err = tasks.DueToday(ctx, conn)
		}
		if err != nil {
			return err
		}
		if len(ts) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "no tasks")
			return nil
		}

		// Each line: ID\tprimark+content\tproject\tdue
		// fzf displays fields 2..N (ID stays hidden, used to identify selection).
		var input strings.Builder
		for _, t := range ts {
			due := formatDue(t.DueDate, t.DueDatetime)
			fmt.Fprintf(&input, "%s\t%s%s\t%s\t%s\n",
				t.ID, priorityMark(t.Priority), t.Content, t.ProjectName, due)
		}

		fzf := exec.CommandContext(ctx, "fzf",
			"--delimiter=\t",
			"--with-nth=2..",
			"--height=40%",
			"--layout=reverse",
			"--no-multi",
		)
		fzf.Stdin = strings.NewReader(input.String())
		fzf.Stderr = os.Stderr
		var out bytes.Buffer
		fzf.Stdout = &out

		if err := fzf.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
				// User cancelled fzf (ESC / Ctrl-C) — silent exit.
				return nil
			}
			return err
		}

		selected := strings.TrimSpace(out.String())
		if selected == "" {
			return nil
		}
		id := strings.SplitN(selected, "\t", 2)[0]
		fmt.Fprintln(cmd.OutOrStdout(), id)
		return nil
	},
}

func init() {
	pickCmd.Flags().StringArrayVarP(&pickLabels, "label", "l", nil, "filter by label (repeatable, AND logic)")
	pickCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	root.AddCommand(pickCmd)
}
