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

			// second header line: project/section and labels
			loc := t.ProjectName
			if t.SectionName != "" {
				loc += " / " + t.SectionName
			}
			lbls, _ := tasks.Labels(ctx, conn, t.ID)
			var meta []string
			if loc != "" {
				meta = append(meta, loc)
			}
			if len(lbls) > 0 {
				meta = append(meta, strings.Join(lbls, ", "))
			}
			if len(meta) > 0 {
				fmt.Fprintf(out, "  %s\n", strings.Join(meta, "  ·  "))
			}

			acted := false
			for !acted {
				fmt.Fprintf(out, "  d = done  r <date> = reschedule  s = skip  v = details  q = quit\n> ")
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
					updated, err := client.UpdateTaskFields(ctx, t.ID, map[string]any{"due_string": dateStr})
					if err != nil {
						fmt.Fprintln(out, "  ! invalid date — try again")
						continue
					}
					conn.ExecContext(ctx, `UPDATE tasks SET due_date=NULL, due_string=? WHERE id=?`, dateStr, t.ID)
					if updated.Due == nil {
						fmt.Fprintln(out, "  → due date cleared")
					} else {
						label := updated.Due.Date
						if updated.Due.String != "" && (updated.Due.IsRecurring || updated.Due.String != dateStr) {
							label = updated.Due.String + "  ·  " + updated.Due.Date
						}
						fmt.Fprintf(out, "  → %s\n", label)
					}
					nRescheduled++
					acted = true

				case input == "s" || input == "":
					fmt.Fprintln(out, "  → skipped")
					nSkipped++
					acted = true

				case input == "v":
					var desc string
					conn.QueryRowContext(ctx, `SELECT description FROM tasks WHERE id=?`, t.ID).Scan(&desc)
					if desc != "" {
						fmt.Fprintf(out, "  desc  %s\n", desc)
					}
					if t.URL != "" {
						fmt.Fprintf(out, "  url   %s\n", t.URL)
					}
					if comments, err := client.GetComments(ctx, t.ID); err == nil && len(comments) > 0 {
						fmt.Fprintln(out, "  comments")
						for _, c := range comments {
							fmt.Fprintf(out, "    %s  %s\n", c.PostedAt[:10], c.Content)
						}
					}
					// acted stays false — reprompt for the same task

				case input == "q":
					quit = true
					acted = true

				default:
					fmt.Fprintln(out, "  ! unknown — try: d, r <date>, s, v, q")
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
