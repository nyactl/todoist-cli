package main

import (
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
