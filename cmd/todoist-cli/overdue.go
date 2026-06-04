package main

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var overdueCmd = &cobra.Command{
	Use:               "overdue",
	Short:             "Triage overdue tasks interactively",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		out := cmd.OutOrStdout()

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		ts, err := tasks.Overdue(ctx, conn)
		if err != nil {
			return err
		}
		if len(ts) == 0 {
			fmt.Fprintln(out, "no overdue tasks")
			return nil
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		fmt.Fprintf(out, "%d overdue task(s)\n", len(ts))

		var nDone, nRescheduled, nSkipped int
		scanner := bufio.NewScanner(cmd.InOrStdin())
		quit := false

		for i, t := range ts {
			if quit {
				break
			}
			overdue := formatDue(t.DueDate, t.DueDatetime)
			fmt.Fprintf(out, "\n[%d/%d]  %s  (%s)\n", i+1, len(ts), truncate(t.Content, 50), overdue)

			acted := false
			for !acted {
				fmt.Fprintf(out, "  d = done  r <date> = reschedule  s = skip  q = quit\n> ")
				if !scanner.Scan() {
					quit = true
					acted = true
					break
				}
				input := strings.TrimSpace(scanner.Text())

				switch {
				case input == "d":
					if err := client.CloseTask(ctx, t.ID); err != nil {
						fmt.Fprintf(out, "  ! %v\n", err)
						continue
					}
					conn.ExecContext(ctx, `UPDATE tasks SET is_completed=1 WHERE id=?`, t.ID)
					fmt.Fprintln(out, "  ✓ done")
					nDone++
					acted = true

				case strings.HasPrefix(input, "r "):
					dateStr := strings.TrimSpace(strings.TrimPrefix(input, "r "))
					if dateStr == "" {
						fmt.Fprintln(out, "  ! specify a date, e.g.: r tomorrow")
						continue
					}
					if _, err := client.UpdateTaskFields(ctx, t.ID, map[string]any{"due_string": dateStr}); err != nil {
						fmt.Fprintln(out, "  ! invalid date — try again")
						continue
					}
					conn.ExecContext(ctx, `UPDATE tasks SET due_date=NULL, due_string=? WHERE id=?`, dateStr, t.ID)
					fmt.Fprintln(out, "  → rescheduled")
					nRescheduled++
					acted = true

				case input == "s" || input == "":
					fmt.Fprintln(out, "  → skipped")
					nSkipped++
					acted = true

				case input == "q":
					quit = true
					acted = true

				default:
					fmt.Fprintln(out, "  ! unknown — try: d, r <date>, s, q")
				}
			}
		}

		fmt.Fprintf(out, "\n%d done · %d rescheduled · %d skipped\n", nDone, nRescheduled, nSkipped)
		return nil
	},
}

func init() {
	root.AddCommand(overdueCmd)
}
