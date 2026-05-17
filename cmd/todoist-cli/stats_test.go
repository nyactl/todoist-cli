package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// assertStat checks that the output contains a line with both the label and the count.
func assertStat(t *testing.T, out, label string, count int) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) {
			if strings.Contains(line, fmt.Sprintf("%d", count)) {
				return
			}
			t.Errorf("line with %q: expected count %d, got: %q", label, count, line)
			return
		}
	}
	t.Errorf("no line with %q found in output:\n%s", label, out)
}

func TestStats_EmptyDB(t *testing.T) {
	newTestEnv(t, nil)

	out, err := runCmd(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, label := range []string{"overdue", "due today", "due this week", "open total"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected label %q in output, got: %q", label, out)
		}
	}
}

func TestStats_CountsOverdue(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Late', 'p1', date('now', '-2 day'))`)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t2', 'Also late', 'p1', date('now', '-1 day'))`)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t3', 'Today', 'p1', date('now'))`)

	out, err := runCmd(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	assertStat(t, out, "overdue", 2)
	assertStat(t, out, "due today", 1)
}

func TestStats_CountsDueThisWeek(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Soon', 'p1', date('now', '+3 day'))`)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t2', 'Far future', 'p1', date('now', '+14 day'))`)

	out, err := runCmd(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	assertStat(t, out, "due this week", 1)
}

func TestStats_OpenTotal_ExcludesCompleted(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Task A", "p1", "")
	hSeedTask(t, env.conn, "t2", "Task B", "p1", "")
	hSeedTask(t, env.conn, "t3", "Task C", "p1", "")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, is_completed) VALUES ('t4', 'Done', 'p1', 1)`)

	out, err := runCmd(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	assertStat(t, out, "open total", 3)
}

func TestStats_WithProjectContext_FiltersToProject(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "t1", "Work task", "p1", "")
	hSeedTask(t, env.conn, "t2", "Personal task A", "p2", "")
	hSeedTask(t, env.conn, "t3", "Personal task B", "p2", "")

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if !strings.Contains(out, "Work") {
		t.Errorf("expected project name in output, got: %q", out)
	}
	assertStat(t, out, "open total", 1)
}
