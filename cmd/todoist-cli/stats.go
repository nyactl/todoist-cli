package main

import (
	"fmt"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:               "stats",
	Short:             "Show task counts for overdue, today, this week and completed",
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
		projectFilter := ""
		if st.HasProject() {
			projectFilter = st.ProjectID
		}

		var overdue, dueToday, dueWeek, openTotal int

		base := `SELECT COUNT(*) FROM tasks WHERE is_completed = 0 AND (parent_id IS NULL OR parent_id = '')`
		proj := ""
		if projectFilter != "" {
			proj = ` AND project_id = ?`
		}
		arg := func() []any {
			if projectFilter != "" {
				return []any{projectFilter}
			}
			return nil
		}

		today := time.Now().Format("2006-01-02")
		weekEnd := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

		if err := conn.QueryRowContext(ctx,
			base+` AND due_date IS NOT NULL AND due_date < ?`+proj,
			append([]any{today}, arg()...)...).Scan(&overdue); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx,
			base+` AND due_date = ?`+proj,
			append([]any{today}, arg()...)...).Scan(&dueToday); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx,
			base+` AND due_date > ? AND due_date <= ?`+proj,
			append([]any{today, weekEnd}, arg()...)...).Scan(&dueWeek); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx,
			base+proj, arg()...).Scan(&openTotal); err != nil {
			return err
		}

		w := cmd.OutOrStdout()
		if st.HasProject() {
			fmt.Fprintf(w, "  project  %s\n\n", st.ProjectName)
		}
		fmt.Fprintf(w, "  %-14s %d\n", "overdue", overdue)
		fmt.Fprintf(w, "  %-14s %d\n", "due today", dueToday)
		fmt.Fprintf(w, "  %-14s %d\n", "due this week", dueWeek)
		fmt.Fprintf(w, "  %-14s %d\n", "open total", openTotal)

		// completed counts — live API call, silently skipped if no token
		token, err := config.GetToken()
		if err != nil {
			return nil
		}
		client := todoist.New(token)

		startOfDay := func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}
		sinceDay := startOfDay(time.Now())
		sinceWeek := startOfDay(time.Now().AddDate(0, 0, -7))

		doneToday, doneWeekCount := 0, 0
		if res, err := client.GetCompletedSince(ctx, sinceWeek, projectFilter); err == nil {
			for _, t := range res.Tasks {
				doneWeekCount++
				if t.CompletedAt >= sinceDay.Format(time.RFC3339)[:10] {
					doneToday++
				}
			}
			fmt.Fprintf(w, "\n  %-14s %d\n", "done today", doneToday)
			fmt.Fprintf(w, "  %-14s %d\n", "done this week", doneWeekCount)
		}

		return nil
	},
}

func init() {
	root.AddCommand(statsCmd)
}
