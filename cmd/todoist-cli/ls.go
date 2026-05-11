package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"todoist-cli/internal/config"
	"todoist-cli/internal/db"
	"todoist-cli/internal/tasks"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	lsDone   string
	lsLabels []string
)

var lsCmd = &cobra.Command{
	Use:               "ls",
	Short:             "List tasks",
	ValidArgsFunction: cobra.NoFileCompletions,
	Long: `List tasks. Without a project context shows today's and overdue tasks across all projects.
With a context (set via cd) shows all active tasks in that project by section.

Use --done [period] to review completed tasks (live API call).
Period: today, week, month, year, Nd/Nw/Nm (e.g. 7d, 2w, 3m). Defaults to today.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if lsDone != "" {
			return runLsDone(cmd)
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

		if len(lsLabels) > 0 {
			projectID := ""
			if st.HasProject() {
				projectID = st.ProjectID
			}
			ts, err := tasks.ByLabels(ctx, conn, lsLabels, projectID)
			if err != nil {
				return err
			}
			if len(ts) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			printByProject(ts)
			return nil
		}

		if st.HasProject() {
			ts, err := tasks.ByProject(ctx, conn, st.ProjectID)
			if err != nil {
				return err
			}
			if len(ts) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			printBySection(ts)
		} else {
			ts, err := tasks.DueToday(ctx, conn)
			if err != nil {
				return err
			}
			if len(ts) == 0 {
				fmt.Println("nothing due")
				return nil
			}
			printByProject(ts)
		}
		return nil
	},
}

func runLsDone(cmd *cobra.Command) error {
	since, err := parseSince(lsDone)
	if err != nil {
		return err
	}
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
	token, err := config.GetToken()
	if err != nil {
		return err
	}
	client := todoist.New(token)
	res, err := client.GetCompletedSince(ctx, since, st.ProjectID)
	if err != nil {
		return fmt.Errorf("fetch completed: %w", err)
	}

	items := res.Tasks
	if len(lsLabels) > 0 {
		items, err = filterCompletedByLabels(ctx, conn, items, lsLabels)
		if err != nil {
			return err
		}
	}
	if len(items) == 0 {
		fmt.Println("nothing completed")
		return nil
	}

	if st.HasProject() {
		for _, t := range items {
			printCompleted(t)
		}
		return nil
	}

	type projGroup struct {
		name  string
		items []todoist.CompletedTask
	}
	groupMap := map[string]*projGroup{}
	var order []string
	for _, t := range items {
		if _, ok := groupMap[t.ProjectID]; !ok {
			name := res.ProjectName[t.ProjectID]
			if name == "" {
				name = "(no project)"
			}
			groupMap[t.ProjectID] = &projGroup{name: name}
			order = append(order, t.ProjectID)
		}
		groupMap[t.ProjectID].items = append(groupMap[t.ProjectID].items, t)
	}
	first := true
	for _, pid := range order {
		g := groupMap[pid]
		if !first {
			fmt.Println()
		}
		first = false
		fmt.Printf("  %s\n", g.name)
		for _, t := range g.items {
			printCompleted(t)
		}
	}
	return nil
}

func filterCompletedByLabels(ctx context.Context, db *sql.DB, items []todoist.CompletedTask, labelNames []string) ([]todoist.CompletedTask, error) {
	if len(items) == 0 || len(labelNames) == 0 {
		return items, nil
	}
	idPH := make([]string, len(items))
	labelPH := make([]string, len(labelNames))
	args := make([]any, 0, len(items)+len(labelNames))
	for i, t := range items {
		idPH[i] = "?"
		args = append(args, t.TaskID)
	}
	for i, l := range labelNames {
		labelPH[i] = "?"
		args = append(args, l)
	}
	q := fmt.Sprintf(
		`SELECT task_id FROM task_labels
		 WHERE task_id IN (%s) AND label_name IN (%s)
		 GROUP BY task_id HAVING COUNT(DISTINCT label_name) = %d`,
		strings.Join(idPH, ","), strings.Join(labelPH, ","), len(labelNames))
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matched := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		matched[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := items[:0]
	for _, t := range items {
		if matched[t.TaskID] {
			out = append(out, t)
		}
	}
	return out, nil
}

func parseSince(s string) (time.Time, error) {
	now := time.Now()
	startOfDay := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	switch s {
	case "today", "":
		return startOfDay(now), nil
	case "week":
		return startOfDay(now.AddDate(0, 0, -7)), nil
	case "month":
		return startOfDay(now.AddDate(0, -1, 0)), nil
	case "year":
		return startOfDay(now.AddDate(-1, 0, 0)), nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("unknown period %q — use: today, week, month, year, Nd, Nw, Nm", s)
	}
	unit := s[len(s)-1]
	var n int
	if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("unknown period %q — use: today, week, month, year, Nd, Nw, Nm", s)
	}
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -n*7), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q — use d (days), w (weeks), m (months)", string(unit))
	}
}

func printCompleted(t todoist.CompletedTask) {
	at := formatCompletedAt(t.CompletedAt)
	id := shortID(t.TaskID)
	content := truncate(t.Content, 40)
	fmt.Printf("  ✓  %s  %-40s  %s\n", id, content, at)
}

func printByProject(ts []tasks.Task) {
	groups := map[string][]tasks.Task{}
	var order []string
	for _, t := range ts {
		key := t.ProjectName
		if key == "" {
			key = "(no project)"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	first := true
	for _, proj := range order {
		if !first {
			fmt.Println()
		}
		first = false
		fmt.Printf("  %s\n", proj)
		for _, t := range groups[proj] {
			printTask(t)
		}
	}
}

func printBySection(ts []tasks.Task) {
	groups := map[string][]tasks.Task{}
	var order []string
	for _, t := range ts {
		if _, ok := groups[t.SectionName]; !ok {
			order = append(order, t.SectionName)
		}
		groups[t.SectionName] = append(groups[t.SectionName], t)
	}
	first := true
	for _, sec := range order {
		if !first {
			fmt.Println()
		}
		first = false
		if sec != "" {
			fmt.Printf("  %s\n", sec)
		}
		for _, t := range groups[sec] {
			printTask(t)
		}
	}
}

func printTask(t tasks.Task) {
	pri := priorityMark(t.Priority)
	id := shortID(t.ID)
	due := formatDue(t.DueDate, t.DueDatetime)
	content := truncate(t.Content, 40)
	fmt.Printf("  %s  %s  %-40s  %s\n", pri, id, content, due)
}

func priorityMark(p int) string {
	switch p {
	case 4:
		return "!!"
	case 3:
		return "! "
	case 2:
		return "· "
	default:
		return "  "
	}
}

func shortID(id string) string {
	if len(id) <= 4 {
		return fmt.Sprintf("%-4s", id)
	}
	return id[:4]
}

func formatDue(date, datetime string) string {
	if date == "" {
		return ""
	}
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	switch {
	case date == today:
		return "today"
	case date == tomorrow:
		return "tomorrow"
	case date < today:
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return date
		}
		days := int(time.Since(t).Hours() / 24)
		return fmt.Sprintf("overdue %dd", days)
	default:
		return date
	}
}

func formatCompletedAt(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05.000000Z", iso)
	if err != nil {
		t, err = time.Parse(time.RFC3339, iso)
		if err != nil {
			return iso
		}
	}
	today := time.Now().Format("2006-01-02")
	if t.Local().Format("2006-01-02") == today {
		return t.Local().Format("15:04")
	}
	return t.Local().Format("01-02 15:04")
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func periodCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"today", "week", "month", "year", "1d", "7d", "30d", "1w", "2w", "1m"},
		cobra.ShellCompDirectiveNoFileComp
}


func taskCompleter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	conn, err := db.Open()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer conn.Close()
	ctx := cmd.Context()

	var projectID string
	if flagProject, err := cmd.Flags().GetString("project"); err == nil && flagProject != "" {
		if id, err := tasks.ProjectByName(ctx, conn, flagProject); err == nil {
			projectID = id
		}
	}
	if projectID == "" {
		if st, _ := loadContext(ctx, conn); st.HasProject() {
			projectID = st.ProjectID
		}
	}

	var ts []tasks.Task
	if projectID != "" {
		ts, _ = tasks.ByProject(ctx, conn, projectID)
	} else {
		ts, _ = tasks.DueToday(ctx, conn)
	}
	completions := make([]string, len(ts))
	for i, t := range ts {
		completions[i] = t.Content + "\t" + t.ID
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	lsCmd.Flags().StringVar(&lsDone, "done", "", "show completed tasks: today, week, month, year, Nd/Nw/Nm")
	lsCmd.Flags().StringArrayVarP(&lsLabels, "label", "l", nil, "filter by label (repeatable, AND logic)")
	lsCmd.RegisterFlagCompletionFunc("done", periodCompleter)
	lsCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	root.AddCommand(lsCmd)
}
