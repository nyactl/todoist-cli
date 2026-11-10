package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	lsDone      string
	lsLabels    []string
	lsNotLabels []string
	lsBoard     bool
	lsPriority  int
	lsRecursive bool
	lsGlobal    bool
	lsProject   string
)

// lsScope is the project one ls invocation is limited to: the -p project, or
// the active cd context unless --global drops it.
type lsScope struct {
	projectID   string
	projectName string
	explicit    bool // from -p rather than the stored context
}

func (s lsScope) scoped() bool { return s.projectID != "" }

// note explains an empty scoped result. Only a cd context gets the hint on
// how to clear it; -p lasts for this call alone.
func (s lsScope) note() string {
	if s.explicit {
		return fmt.Sprintf("in %q", s.projectName)
	}
	return contextScopeNote(s.projectName)
}

// resolveLsScope never writes the stored context for -p, so scripts can look
// at one project without saving and restoring their cd state (issue #41).
func resolveLsScope(ctx context.Context, conn *sql.DB) (lsScope, error) {
	if lsProject != "" {
		id, err := tasks.ProjectByName(ctx, conn, lsProject)
		if err != nil {
			return lsScope{}, err
		}
		name, err := tasks.ProjectName(ctx, conn, id)
		if err != nil {
			return lsScope{}, err
		}
		return lsScope{projectID: id, projectName: name, explicit: true}, nil
	}
	st, err := loadContext(ctx, conn)
	if err != nil {
		return lsScope{}, err
	}
	if !st.HasProject() || lsGlobal {
		return lsScope{}, nil
	}
	return lsScope{projectID: st.ProjectID, projectName: st.ProjectName}, nil
}

// contextScopeNote explains that an empty (or partial) result came from the
// active project context and how to widen the search — so a context-scoped
// query is never mistaken for an account-wide one (issue #28), mirroring the
// --done cue from #16.
func contextScopeNote(projectName string) string {
	return fmt.Sprintf("in %q — run 'todoist-cli cd' to clear the context (or pass --global) to search all projects", projectName)
}

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

-g/--global ignores the active project context for this one invocation, so label,
priority and --done queries run account-wide without changing your cd context.

-p/--project <name|id> lists that project for this one invocation as if it were
the active context, without changing your cd context. It composes with the other
flags and cannot be combined with --global.

Use --done [period] to review completed tasks (live API call).
Period: today, week, month, year, Nd/Nw/Nm (e.g. 7d, 2w, 3m). Defaults to today.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if lsGlobal && lsProject != "" {
			return fmt.Errorf("--global and --project cannot be combined")
		}
		if lsDone != "" {
			return runLsDone(cmd)
		}
		if lsPriority != 0 && (lsPriority < 1 || lsPriority > 4) {
			return fmt.Errorf("priority must be between 1 and 4")
		}
		if lsRecursive && lsBoard {
			return fmt.Errorf("--recursive and --board cannot be combined")
		}
		// --board and --recursive act on the active project; with --global there
		// is no project to act on, so the combination is contradictory.
		if lsGlobal && lsBoard {
			return fmt.Errorf("--global and --board cannot be combined")
		}
		if lsGlobal && lsRecursive {
			return fmt.Errorf("--global and --recursive cannot be combined")
		}
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx := cmd.Context()
		sc, err := resolveLsScope(ctx, conn)
		if err != nil {
			return err
		}
		scoped := sc.scoped()

		if len(lsLabels) > 0 || len(lsNotLabels) > 0 {
			if err := validateLabels(ctx, conn, append(append([]string{}, lsLabels...), lsNotLabels...)...); err != nil {
				return err
			}
			ts, err := tasks.ByLabelFilter(ctx, conn, lsLabels, lsNotLabels, sc.projectID)
			if err != nil {
				return err
			}
			ts = filterTasksByPriority(ts, lsPriority)
			if jsonOut {
				return emitListJSON(ctx, conn, ts, sc)
			}
			if len(ts) == 0 {
				if scoped {
					fmt.Printf("no tasks %s\n", sc.note())
				} else {
					fmt.Println("no tasks")
				}
				return nil
			}
			printByProject(ts)
			return nil
		}

		if scoped {
			var ts []tasks.Task
			if lsRecursive {
				ts, err = tasks.BySubtree(ctx, conn, sc.projectID)
			} else {
				ts, err = tasks.ByProject(ctx, conn, sc.projectID)
			}
			if err != nil {
				return err
			}
			ts = filterTasksByPriority(ts, lsPriority)
			if jsonOut {
				return emitListJSON(ctx, conn, ts, sc)
			}
			if len(ts) == 0 {
				fmt.Printf("no tasks %s\n", sc.note())
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
			if jsonOut {
				return emitListJSON(ctx, conn, ts, lsScope{})
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

// emitListJSON prints ts as a JSON array. The context hint the text view
// prints for an empty scoped result goes to stderr so stdout stays valid JSON.
func emitListJSON(ctx context.Context, conn *sql.DB, ts []tasks.Task, sc lsScope) error {
	if len(ts) == 0 && sc.scoped() {
		fmt.Fprintf(os.Stderr, "no tasks %s\n", sc.note())
	}
	return emitTasksJSON(ctx, conn, ts)
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

	sc, err := resolveLsScope(ctx, conn)
	if err != nil {
		return err
	}
	scoped := sc.scoped()
	token, err := config.GetToken()
	if err != nil {
		return err
	}
	client := todoist.New(token)
	items, err := client.GetCompleted(ctx, since, completedUntil(), sc.projectID)
	if err != nil {
		return fmt.Errorf("fetch completed: %w", err)
	}
	projectName, err := cachedNames(ctx, conn, "projects")
	if err != nil {
		return err
	}
	sectionName, err := cachedNames(ctx, conn, "sections")
	if err != nil {
		return err
	}
	lookupMissingNames(ctx, client, items, projectName, sectionName)

	if len(lsLabels) > 0 || len(lsNotLabels) > 0 {
		if err := validateCompletedLabels(ctx, conn, items, append(append([]string{}, lsLabels...), lsNotLabels...)...); err != nil {
			return err
		}
		items = filterCompletedByLabels(items, lsLabels, lsNotLabels)
	}
	if jsonOut {
		if len(items) == 0 && scoped {
			fmt.Fprintf(os.Stderr, "nothing completed %s\n", sc.note())
		}
		type jsonCompleted struct {
			ID          string  `json:"id"`
			Content     string  `json:"content"`
			Project     *string `json:"project"`
			Section     *string `json:"section"`
			CompletedAt string  `json:"completed_at"`
		}
		out := make([]jsonCompleted, 0, len(items))
		for _, t := range items {
			out = append(out, jsonCompleted{
				ID:          t.TaskID,
				Content:     t.Content,
				Project:     nilIfEmpty(projectName[t.ProjectID]),
				Section:     nilIfEmpty(sectionName[t.SectionID]),
				CompletedAt: t.CompletedAt,
			})
		}
		return emitJSON(out)
	}
	if len(items) == 0 {
		// Completed queries are scoped to the active project context. Without a
		// cue, an empty result reads as "completed tasks are broken" when really
		// the context just has none (issue #16).
		if scoped {
			fmt.Printf("nothing completed %s\n", sc.note())
		} else {
			fmt.Println("nothing completed")
		}
		return nil
	}

	ids := make([]string, len(items))
	for i, t := range items {
		ids[i] = t.TaskID
	}
	short := shortIDs(ctx, ids...)
	width := 0
	for _, id := range short {
		width = max(width, len(id))
	}

	if scoped {
		for _, t := range items {
			printCompleted(t, short, width)
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
			name := projectName[t.ProjectID]
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
			printCompleted(t, short, width)
		}
	}
	return nil
}

// completedUntil ends every completed-tasks query a little after now, so a
// task completed a moment ago is not lost to clock skew with the server.
func completedUntil() time.Time {
	return time.Now().Add(time.Hour)
}

// cachedNames maps the IDs of a cached table (projects or sections) to names.
func cachedNames(ctx context.Context, conn *sql.DB, table string) (map[string]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT id, name FROM `+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

// lookupMissingNames fills in the names of projects and sections that are not
// cached — archived or deleted since the tasks were completed — by asking the
// API once per ID. Deleted ones stay unnamed.
func lookupMissingNames(ctx context.Context, client *todoist.Client, items []todoist.CompletedTask, projectName, sectionName map[string]string) {
	tried := map[string]bool{}
	for _, t := range items {
		if id := t.ProjectID; id != "" && projectName[id] == "" && !tried["p"+id] {
			tried["p"+id] = true
			if p, err := client.GetProject(ctx, id); err == nil {
				projectName[id] = p.Name
			}
		}
		if id := t.SectionID; id != "" && sectionName[id] == "" && !tried["s"+id] {
			tried["s"+id] = true
			if sec, err := client.GetSection(ctx, id); err == nil {
				sectionName[id] = sec.Name
			}
		}
	}
}

// validateCompletedLabels accepts a label that is known to the cache or
// carried by one of the completed tasks: a shared label can live only on
// completed tasks, which a sync does not cache.
func validateCompletedLabels(ctx context.Context, conn *sql.DB, items []todoist.CompletedTask, names ...string) error {
	onItems := map[string]bool{}
	for _, t := range items {
		for _, l := range t.Labels {
			onItems[l] = true
		}
	}
	var unknown []string
	for _, n := range names {
		if !onItems[n] {
			unknown = append(unknown, n)
		}
	}
	return validateLabels(ctx, conn, unknown...)
}

// filterCompletedByLabels keeps the tasks carrying every include label and
// none of the exclude labels.
func filterCompletedByLabels(items []todoist.CompletedTask, include, exclude []string) []todoist.CompletedTask {
	var out []todoist.CompletedTask
	for _, t := range items {
		if containsAll(t.Labels, include) && !containsAny(t.Labels, exclude) {
			out = append(out, t)
		}
	}
	return out
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

func containsAny(have, want []string) bool {
	for _, w := range want {
		if slices.Contains(have, w) {
			return true
		}
	}
	return false
}

var relativePeriod = regexp.MustCompile(`^([0-9]+)([a-z])$`)

// parseSince turns a --done period into the start of the window. Every period
// starts at local midnight, so 7d and 1w cover the same days as week.
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
	m := relativePeriod.FindStringSubmatch(s)
	n := 0
	if m != nil {
		n, _ = strconv.Atoi(m[1])
	}
	if n <= 0 {
		return time.Time{}, fmt.Errorf("unknown period %q — use: today, week, month, year, Nd, Nw, Nm", s)
	}
	switch m[2] {
	case "d":
		return startOfDay(now.AddDate(0, 0, -n)), nil
	case "w":
		return startOfDay(now.AddDate(0, 0, -n*7)), nil
	case "m":
		return startOfDay(now.AddDate(0, -n, 0)), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q — use d (days), w (weeks), m (months)", m[2])
	}
}

func printCompleted(t todoist.CompletedTask, short map[string]string, width int) {
	at := formatCompletedAt(t.CompletedAt)
	// --ids upgrades to the full, unambiguous ID for scripting; otherwise the
	// short prefix keeps the human view compact.
	id := fmt.Sprintf("%-*s", width, short[t.TaskID])
	if showIDs {
		id = t.TaskID
	}
	content := padRight(truncate(t.Content, 40), 40)
	fmt.Printf("  ✓  %s  %s  %s\n", id, content, at)
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
	content := padRight(truncate(t.Content, 50), 50)
	if showIDs {
		// Full ID (not the short prefix) so the output is an unambiguous handle
		// for scripting — shortID can collide across the full cache.
		fmt.Printf("  %s  %s  %s  %s\n", t.ID, pri, content, due)
		return
	}
	fmt.Printf("  %s  %s  %s\n", pri, content, due)
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

// shortIDs maps each id to its shortest unambiguous prefix in the local
// cache, falling back to the first 4 characters when the cache can't be read.
func shortIDs(ctx context.Context, ids ...string) map[string]string {
	if conn, err := db.Open(); err == nil {
		defer conn.Close()
		if m, err := tasks.ShortIDs(ctx, conn, ids...); err == nil {
			return m
		}
	}
	m := make(map[string]string, len(ids))
	for _, id := range ids {
		m[id] = id[:min(4, len(id))]
	}
	return m
}

func shortID(ctx context.Context, id string) string {
	return shortIDs(ctx, id)[id]
}

func formatDue(date, datetime string) string {
	if date == "" {
		return ""
	}
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	clock := ""
	if datetime != "" {
		if c := todoist.LocalClock(datetime); c != "" {
			clock = " " + c
		}
	}
	switch {
	case date == today:
		return "today" + clock
	case date == tomorrow:
		return "tomorrow" + clock
	case date < today:
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return date
		}
		n := time.Now()
		days := int(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).Sub(t).Hours() / 24)
		return fmt.Sprintf("overdue %dd", days)
	default:
		return date + clock
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

// truncate shortens s to at most n terminal columns, ending in "…" when cut.
// Widths are display columns, so umlauts count once and CJK characters twice.
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return runewidth.Truncate(s, n, "…")
}

// padRight pads s with spaces to n terminal columns; fmt's %-*s pads bytes.
func padRight(s string, n int) string {
	return runewidth.FillRight(s, n)
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

// terminalWidth prefers $COLUMNS (shells rarely export it, so it acts as an
// override), then the size of the terminal on stdout, then 120 for pipes.
func terminalWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var n int
		if _, err := fmt.Sscanf(cols, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
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
	colWidth := (width - runewidth.StringWidth(sep)*(len(cols)-1)) / len(cols)
	if colWidth < 20 {
		colWidth = 20
	}

	// header
	for i, col := range cols {
		if i > 0 {
			fmt.Print(sep)
		}
		fmt.Print(padRight(truncate(col.name, colWidth), colWidth))
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
				cell := priorityMark(t.Priority) + " " + truncate(t.Content, contentWidth)
				fmt.Print(padRight(cell, colWidth))
			} else {
				fmt.Print(strings.Repeat(" ", colWidth))
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
	lsCmd.Flags().BoolVar(&jsonOut, "json", false, "print tasks as a JSON array (ignores --board and --ids)")
	lsCmd.Flags().BoolVarP(&lsGlobal, "global", "g", false, "ignore the active project context; search all projects")
	lsCmd.Flags().StringVarP(&lsProject, "project", "p", "", "list this project (name or ID) without changing the active context")
	lsCmd.RegisterFlagCompletionFunc("done", periodCompleter)
	lsCmd.RegisterFlagCompletionFunc("label", labelCompleter)
	lsCmd.RegisterFlagCompletionFunc("not-label", labelCompleter)
	lsCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	root.AddCommand(lsCmd)
}
