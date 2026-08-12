package main

import (
	"context"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func TestLs_NothingDue(t *testing.T) {
	newTestEnv(t, nil)

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "nothing due") {
		t.Errorf("expected 'nothing due', got: %q", out)
	}
}

func TestLs_ShowsDueTodayTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Buy milk', 'p1', date('now'))`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t2', 'Overdue task', 'p1', date('now', '-1 day'))`)

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Buy milk") {
		t.Errorf("expected 'Buy milk' in output, got: %q", out)
	}
	if !strings.Contains(out, "Overdue task") {
		t.Errorf("expected 'Overdue task' in output, got: %q", out)
	}
}

func TestLs_FutureDueNotShown(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Future task', 'p1', date('now', '+5 day'))`)

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Future task") {
		t.Errorf("future task should not appear in ls output, got: %q", out)
	}
}

func TestLs_WithProjectContext_ShowsAllTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Code review", "p1", "")
	hSeedTask(t, env.conn, "t2", "Write tests", "p1", "")

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Code review") {
		t.Errorf("expected 'Code review', got: %q", out)
	}
	if !strings.Contains(out, "Write tests") {
		t.Errorf("expected 'Write tests', got: %q", out)
	}
}

func TestLs_WithProjectContext_NoTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no tasks") {
		t.Errorf("expected 'no tasks', got: %q", out)
	}
}

func TestLs_WithProjectContext_GroupedBySection(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedSection(t, env.conn, "s2", "In Progress", "p1", 1)
	hSeedTask(t, env.conn, "t1", "Plan feature", "p1", "s1")
	hSeedTask(t, env.conn, "t2", "Write PR", "p1", "s2")

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Backlog") {
		t.Errorf("expected section 'Backlog', got: %q", out)
	}
	if !strings.Contains(out, "In Progress") {
		t.Errorf("expected section 'In Progress', got: %q", out)
	}
	backlogPos := strings.Index(out, "Backlog")
	progressPos := strings.Index(out, "In Progress")
	if backlogPos > progressPos {
		t.Errorf("Backlog should appear before In Progress in output")
	}
}

func TestLs_BoardFlag(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedSection(t, env.conn, "s2", "Done", "p1", 1)
	hSeedTask(t, env.conn, "t1", "Task A", "p1", "s1")
	hSeedTask(t, env.conn, "t2", "Task B", "p1", "s2")

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "-b")
	if err != nil {
		t.Fatal(err)
	}
	// Board view uses │ as separator
	if !strings.Contains(out, "│") {
		t.Errorf("expected board separator │, got: %q", out)
	}
	if !strings.Contains(out, "Backlog") || !strings.Contains(out, "Done") {
		t.Errorf("expected both column headers, got: %q", out)
	}
}

func TestLs_LabelFilter(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Urgent task", "p1", "")
	hSeedTask(t, env.conn, "t2", "Normal task", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'due_date)`)

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "-l", "urgent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Urgent task") {
		t.Errorf("expected 'Urgent task', got: %q", out)
	}
	if strings.Contains(out, "Normal task") {
		t.Errorf("'Normal task' should not appear when filtering by label, got: %q", out)
	}
}

// Regression guard for #13: `ls -l <label>` with no project context must
// search all active tasks account-wide, including undated ones — not silently
// scope to the today+overdue agenda.
func TestLs_LabelFilter_NoContext_FindsUndatedAcrossProjects(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Home")
	// Two undated tasks carrying the label, in different projects.
	hSeedTask(t, env.conn, "t1", "wip in work", "p1", "")
	hSeedTask(t, env.conn, "t2", "wip at home", "p2", "")
	hSeedTask(t, env.conn, "t3", "unrelated", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'wip')`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2', 'wip')`)

	// Ensure there is no active project context.
	if err := state.Save(&state.State{}); err != nil {
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "-l", "wip")
	if err != nil {
		t.Fatalf("ls -l: %v", err)
	}
	if !strings.Contains(out, "wip in work") || !strings.Contains(out, "wip at home") {
		t.Errorf("expected both undated labelled tasks across projects, got: %q", out)
	}
	if strings.Contains(out, "unrelated") {
		t.Errorf("unlabelled task should not appear, got: %q", out)
	}
}

func TestLs_PriorityFilter_ShowsOnlyMatchingPriority(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}
	// Seed tasks at different priorities.
	seedTaskWithPriority(t, env, "t1", "Urgent task", "p1", 4)
	seedTaskWithPriority(t, env, "t2", "Normal task", "p1", 1)
	seedTaskWithPriority(t, env, "t3", "High task", "p1", 3)

	out, err := runCmd(t, "ls", "--priority", "4")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "Urgent task") {
		t.Errorf("expected urgent task in output, got: %q", out)
	}
	if strings.Contains(out, "Normal task") {
		t.Errorf("expected normal task to be filtered out, got: %q", out)
	}
	if strings.Contains(out, "High task") {
		t.Errorf("expected high task to be filtered out, got: %q", out)
	}
}

func TestLs_PriorityFilter_NoMatch_ShowsNoTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}
	seedTaskWithPriority(t, env, "t1", "Normal task", "p1", 1)

	out, err := runCmd(t, "ls", "--priority", "4")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "no tasks") {
		t.Errorf("expected 'no tasks' when no priority match, got: %q", out)
	}
}

func TestLs_PriorityFilter_DueTodayView(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	today := "2026-06-04"
	seedTaskWithPriorityAndDue(t, env, "t1", "Urgent due", "p1", 4, today)
	seedTaskWithPriorityAndDue(t, env, "t2", "Normal due", "p1", 1, today)

	out, err := runCmd(t, "ls", "-P", "4")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "Urgent due") {
		t.Errorf("expected urgent task in output, got: %q", out)
	}
	if strings.Contains(out, "Normal due") {
		t.Errorf("expected normal task to be filtered out, got: %q", out)
	}
}

func TestLs_PriorityFilter_CombinedWithLabel(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}
	seedTaskWithPriority(t, env, "t1", "Urgent+urgent label", "p1", 4)
	seedTaskWithPriority(t, env, "t2", "Normal+urgent label", "p1", 1)
	seedTaskWithPriority(t, env, "t3", "Urgent no label", "p1", 4)
	env.conn.ExecContext(context.Background(), `INSERT INTO task_labels VALUES ('t1', 'urgent')`)
	env.conn.ExecContext(context.Background(), `INSERT INTO task_labels VALUES ('t2', 'urgent')`)

	out, err := runCmd(t, "ls", "-l", "urgent", "-P", "4")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "Urgent+urgent label") {
		t.Errorf("expected task matching both label and priority, got: %q", out)
	}
	if strings.Contains(out, "Normal+urgent label") {
		t.Errorf("expected normal priority task filtered out, got: %q", out)
	}
	if strings.Contains(out, "Urgent no label") {
		t.Errorf("expected task without label filtered out, got: %q", out)
	}
}

func TestLs_InvalidPriority_Errors(t *testing.T) {
	newTestEnv(t, nil)
	_, err := runCmd(t, "ls", "--priority", "5")
	if err == nil {
		t.Fatal("expected error for priority 5, got nil")
	}
}

// seedTaskWithPriority inserts a task with a specific priority directly into the DB.
func seedTaskWithPriority(t *testing.T, env *testEnv, id, content, projectID string, priority int) {
	t.Helper()
	if _, err := env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, priority) VALUES (?, ?, ?, ?)`,
		id, content, projectID, priority); err != nil {
		t.Fatalf("seedTaskWithPriority: %v", err)
	}
}

func seedTaskWithPriorityAndDue(t *testing.T, env *testEnv, id, content, projectID string, priority int, due string) {
	t.Helper()
	if _, err := env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, priority, due_date) VALUES (?, ?, ?, ?, ?)`,
		id, content, projectID, priority, due); err != nil {
		t.Fatalf("seedTaskWithPriorityAndDue: %v", err)
	}
}

func TestLs_StaleContext_AutoClears(t *testing.T) {
	newTestEnv(t, nil)
	// Set context to a project that doesn't exist in the DB
	if err := state.Save(&state.State{ProjectID: "ghost-id", ProjectName: "Ghost"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	// Should not error — falls back to due-today view with warning
	_, err := runCmd(t, "ls")
	if err != nil {
		t.Fatalf("expected ls to succeed with stale context, got: %v", err)
	}

	// Context should now be cleared
	st, _ := state.Load()
	if st.HasProject() {
		t.Errorf("expected context to be cleared after stale project detected")
	}
}
