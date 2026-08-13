package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	lsDone      string
	lsLabels    []string
	lsNotLabels []string
	lsBoard     bool
	lsPriority  int
	lsRecursive bool
)

// validateLabels errors on any label name that is neither a personal label nor
// present on a cached task, turning typos and unsupported negation syntax
// (e.g. -l '!someday') into a clear message instead of a silent empty result.
func validateLabels(ctx context.Context, conn *sql.DB, names ...string) error {
	for _, name := range names {
		lt, err := resolveLabel(ctx, conn, name)
		if err != nil {
			return err
		}
		if !lt.personal && lt.count == 0 {
			return fmt.Errorf("unknown label %q — run: todoist-cli sync", name)
		}
	}
	return nil
}

// showIDs is set by the --ids flag on ls and search; printTask prepends the
// full task ID when it's on.
var showIDs bool

var lsCmd = &cobra.Command{
	Use:               "ls",
	Short:             "List tasks",
	ValidArgsFunction: cobra.NoFileCompletions,
	Long: `List tasks. Without a project context shows today's and overdue tasks across all projects.
With a context (set via cd) shows all active tasks in that project by section.

-r/--recursive additionally includes tasks from sub-projects of the active project,
grouped by project. Only affects the with-context view.

-l/--label searches all active tasks regardless of due date (account-wide when no
context is set, or within the active project). It is not limited to the agenda view.
--not-label excludes tasks carrying a label; it composes with -l. An unknown label
name (including unsupported negation like -l '!x') is reported as an error.

Use --done [period] to review completed tasks (live API call).
Period: today, week, month, year, Nd/Nw/Nm (e.g. 7d, 2w, 3m). Defaults to today.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if lsDone != "" {
			return runLsDone(cmd)
		}
		if lsPriority != 0 && (lsPriority < 1 || lsPriority > 4) {
			return fmt.Errorf("priority must be between 1 and 4")
		}
		if lsRecursive && lsBoard {
			return fmt.Errorf("--recursive and --board cannot be combined")
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

		if len(lsLabels) > 0 || len(lsNotLabels) > 0 {
			if err := validateLabels(ctx, conn, append(append([]string{}, lsLabels...), lsNotLabels...)...); err != nil {
				return err
			}
			projectID := ""
			if st.HasProject() {
				projectID = st.ProjectID
			}
			ts, err := tasks.ByLabelFilter(ctx, conn, lsLabels, lsNotLabels, projectID)
			if err != nil {
				return err
			}
			ts = filterTasksByPriority(ts, lsPriority)
			if len(ts) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			printByProject(ts)
			return nil
		}

		if st.HasProject() {
			var ts []tasks.Task
			if lsRecursive {
				ts, err = tasks.BySubtree(ctx, conn, st.ProjectID)
			} else {
				ts, err = tasks.ByProject(ctx, conn, st.ProjectID)
			}
			if err != nil {
				return err
			}
			ts = filterTasksByPriority(ts, lsPriority)
			if len(ts) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			switch {
			case lsRecursive:
				// Tasks span multiple projects — group by project so the
				// wider view stays legible.
				printByProject(ts)
			case lsBoard:
				printBoard(ts)
			default:
				printBySection(ts)
			}
		} else {
			ts, err := tasks.DueToday(ctx, conn)
			if err != nil {
				return err
			}
			ts = filterTasksByPriority(ts, lsPriority)
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
	if len(lsLabels) > 0 || len(lsNotLabels) > 0 {
		if err := validateLabels(ctx, conn, append(append([]string{}, lsLabels...), lsNotLabels...)...); err != nil {
			return err
		}
		items, err = filterCompletedByLabels(ctx, conn, items, lsLabels, lsNotLabels)
		if err != nil {
			return err
		}
	}
	if len(items) == 0 {
		// Completed queries are scoped to the active project context. Without a
		// cue, an empty result reads as "completed tasks are broken" when really
		// the context just has none (issue #16).
		if st.HasProject() {
			fmt.Printf("nothing completed in %q — run 'todoist-cli cd' to clear the context and search all projects\n", st.ProjectName)
		} else {
			fmt.Println("nothing completed")
		}
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

func filterCompletedByLabels(ctx context.Context, db *sql.DB, items []todoist.CompletedTask, include, exclude []string) ([]todoist.CompletedTask, error) {
	if len(items) == 0 || (len(include) == 0 && len(exclude) == 0) {
		return items, nil
	}
	ids := make([]any, len(items))
	idPH := make([]string, len(items))
	for i, t := range items {
		ids[i] = t.TaskID
		idPH[i] = "?"
	}

	var includeMatch, excludeMatch map[string]bool
	var err error
	if len(include) > 0 {
		// among these items, the ones carrying ALL include labels
		includeMatch, err = completedLabelSet(ctx, db, ids, idPH, include,
			fmt.Sprintf(` GROUP BY task_id HAVING COUNT(DISTINCT label_name) = %d`, len(include)))
		if err != nil {
			return nil, err
		}
	}
	if len(exclude) > 0 {
		// the ones carrying ANY exclude label
		excludeMatch, err = completedLabelSet(ctx, db, ids, idPH, exclude, "")
		if err != nil {
			return nil, err
		}
	}

	out := items[:0]
	for _, t := range items {
		if includeMatch != nil && !includeMatch[t.TaskID] {
			continue
		}
		if excludeMatch != nil && excludeMatch[t.TaskID] {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// completedLabelSet returns the task IDs (from the given item IDs) that match
// the labels, with an optional trailing clause (e.g. a HAVING for AND logic).
func completedLabelSet(ctx context.Context, db *sql.DB, ids []any, idPH []string, labels []string, trailing string) (map[string]bool, error) {
	labelPH := make([]string, len(labels))
	args := append([]any{}, ids...)
	for i, l := range labels {
		labelPH[i] = "?"
		args = append(args, l)
	}
	q := fmt.Sprintf(
		`SELECT task_id FROM task_labels WHERE task_id IN (%s) AND label_name IN (%s)%s`,
		strings.Join(idPH, ","), strings.Join(labelPH, ","), trailing)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	return set, rows.Err()
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
	// --ids upgrades to the full, unambiguous ID for scripting; otherwise the
	// short prefix keeps the human view compact.
	id := shortID(t.TaskID)
	if showIDs {
		id = t.TaskID
	}
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
	due := formatDue(t.DueDate, t.DueDatetime)
	content := truncate(t.Content, 50)
	if showIDs {
		// Full ID (not the short prefix) so the output is an unambiguous handle
		// for scripting — shortID can collide across the full cache.
		fmt.Printf("  %s  %s  %-50s  %s\n", t.ID, pri, content, due)
		return
	}
	fmt.Printf("  %s  %-50s  %s\n", pri, content, due)
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

func filterTasksByPriority(ts []tasks.Task, priority int) []tasks.Task {
	if priority == 0 {
		return ts
	}
	out := ts[:0]
	for _, t := range ts {
		if t.Priority == priority {
			out = append(out, t)
		}
	}
	return out
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

func terminalWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var n int
		if _, err := fmt.Sscanf(cols, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return 120
}

func printBoard(ts []tasks.Task) {
	type column struct {
		name  string
		tasks []tasks.Task
	}
	colMap := map[string]*column{}
	var order []string
	for _, t := range ts {
		key := t.SectionName
		if key == "" {
			key = "(no section)"
		}
		if _, ok := colMap[key]; !ok {
			order = append(order, key)
			colMap[key] = &column{name: key}
		}
		colMap[key].tasks = append(colMap[key].tasks, t)
	}

	cols := make([]*column, len(order))
	for i, k := range order {
		cols[i] = colMap[k]
	}

	const sep = " │ "
	width := terminalWidth()
	colWidth := (width - len(sep)*(len(cols)-1)) / len(cols)
	if colWidth < 20 {
		colWidth = 20
	}

	// header
	for i, col := range cols {
		if i > 0 {
			fmt.Print(sep)
		}
		fmt.Printf("%-*s", colWidth, truncate(col.name, colWidth))
	}
	fmt.Println()
	// divider
	for i := range cols {
		if i > 0 {
			fmt.Print(sep)
		}
		fmt.Print(strings.Repeat("─", colWidth))
	}
	fmt.Println()

	// task rows — each column printed at the same row index
	maxRows := 0
	for _, col := range cols {
		if len(col.tasks) > maxRows {
			maxRows = len(col.tasks)
		}
	}
	// pri(2) + space(1) = 3 fixed chars per task cell
	const taskOverhead = 3
	for row := 0; row < maxRows; row++ {
		for i, col := range cols {
			if i > 0 {
				fmt.Print(sep)
			}
			if row < len(col.tasks) {
				t := col.tasks[row]
				contentWidth := colWidth - taskOverhead
				if contentWidth < 4 {
					contentWidth = 4
				}
				cell := fmt.Sprintf("%s %-*s", priorityMark(t.Priority), contentWidth, truncate(t.Content, contentWidth))
				fmt.Printf("%-*s", colWidth, cell)
			} else {
				fmt.Printf("%-*s", colWidth, "")
			}
		}
		fmt.Println()
	}
}

func init() {
	lsCmd.Flags().StringVarP(&lsDone, "done", "d", "", "show completed tasks: today, week, month, year, Nd/Nw/Nm")
	lsCmd.Flags().StringArrayVarP(&lsLabels, "label", "l", nil, "filter by label (repeatable, AND logic)")
	lsCmd.Flags().StringArrayVar(&lsNotLabels, "not-label", nil, "exclude tasks carrying this label (repeatable)")
	lsCmd.Flags().BoolVarP(&lsBoard, "board", "b", false, "show tasks as side-by-side columns (requires project context)")
	lsCmd.Flags().IntVarP(&lsPriority, "priority", "P", 0, "filter by priority 1–4 (1=normal, 4=urgent)")
	lsCmd.Flags().BoolVarP(&lsRecursive, "recursive", "r", false, "include tasks from sub-projects of the active project, grouped by project")
	lsCmd.Flags().BoolVarP(&showIDs, "ids", "i", false, "prepend the full task ID to each line (for scripting)")
	lsCmd.RegisterFlagCompletionFunc("done", periodCompleter)
	lsCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	lsCmd.RegisterFlagCompletionFunc("not-label", labelCompleter)
	root.AddCommand(lsCmd)
}
