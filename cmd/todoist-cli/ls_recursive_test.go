package main

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// seedDevSubtree sets up Dev(p1) > Frontend(c1) with one task in each.
func seedDevSubtree(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Dev")
	hSeedSubproject(t, env.conn, "c1", "Frontend", "p1")
	hSeedTask(t, env.conn, "t-dev", "dev task", "p1", "")
	hSeedTask(t, env.conn, "t-fe", "frontend task", "c1", "")
}

func TestLs_Recursive_IncludesSubprojectTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	seedDevSubtree(t, env)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Dev"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "-r")
	if err != nil {
		t.Fatalf("ls -r: %v", err)
	}
	if !strings.Contains(out, "dev task") || !strings.Contains(out, "frontend task") {
		t.Errorf("expected tasks from Dev and its sub-project, got: %q", out)
	}
	if !strings.Contains(out, "Frontend") {
		t.Errorf("expected sub-project group header in output, got: %q", out)
	}
}

func TestLs_NoRecursive_OnlyDirectTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	seedDevSubtree(t, env)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Dev"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "dev task") {
		t.Errorf("expected the direct task, got: %q", out)
	}
	if strings.Contains(out, "frontend task") {
		t.Errorf("sub-project task must NOT appear without -r (default unchanged), got: %q", out)
	}
}

func TestLs_Recursive_WithBoard_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	seedDevSubtree(t, env)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Dev"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	_, err := runCmd(t, "ls", "-r", "-b")
	if err == nil {
		t.Fatal("expected an error combining --recursive and --board, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("expected mutual-exclusion error, got: %v", err)
	}
}

func TestLs_Recursive_WithPriorityFilter(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Dev")
	hSeedSubproject(t, env.conn, "c1", "Frontend", "p1")
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, priority) VALUES ('t-hi', 'urgent frontend', 'c1', 4)`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, priority) VALUES ('t-lo', 'normal dev', 'p1', 1)`)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Dev"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "-r", "-P", "4")
	if err != nil {
		t.Fatalf("ls -r -P: %v", err)
	}
	if !strings.Contains(out, "urgent frontend") {
		t.Errorf("expected priority-4 sub-project task, got: %q", out)
	}
	if strings.Contains(out, "normal dev") {
		t.Errorf("priority-1 task must be filtered out across the subtree, got: %q", out)
	}
}

func TestLs_Recursive_NoContext_IsNoop(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Dev")
	hSeedSubproject(t, env.conn, "c1", "Frontend", "p1")
	hSeedTask(t, env.conn, "t-fe", "frontend task", "c1", "") // undated
	if err := state.Save(&state.State{}); err != nil {        // no context
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "-r")
	if err != nil {
		t.Fatalf("ls -r without context: %v", err)
	}
	// With no context, -r is a no-op — it falls back to the today+overdue
	// agenda, which excludes this undated sub-project task.
	if strings.Contains(out, "frontend task") {
		t.Errorf("-r without a context should not list undated sub-project tasks, got: %q", out)
	}
}
