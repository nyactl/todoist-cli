package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// setStdin sets root's stdin for the duration of the test and restores it after.
// cobra's cmd.InOrStdin() walks up to root, so setting it here propagates to subcommands.
func setStdin(t *testing.T, input string) {
	t.Helper()
	root.SetIn(strings.NewReader(input))
	t.Cleanup(func() { root.SetIn(nil) })
}

// seedOverdueTask inserts a task with a due date 3 days in the past.
func seedOverdueTask(t *testing.T, env *testEnv, id, content, projectID string) {
	t.Helper()
	if _, err := env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES (?, ?, ?, date('now', '-3 days'))`,
		id, content, projectID); err != nil {
		t.Fatalf("seedOverdueTask: %v", err)
	}
}

func TestOverdue_NoOverdueTasks(t *testing.T) {
	newTestEnv(t, nil)
	setStdin(t, "")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "no overdue tasks") {
		t.Errorf("expected 'no overdue tasks', got: %q", out)
	}
}

func TestOverdue_DoneAction(t *testing.T) {
	var closeCalled bool
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/close") {
			closeCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Fix login bug", "p1")
	setStdin(t, "d\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !closeCalled {
		t.Error("expected CloseTask API call")
	}
	if !strings.Contains(out, "✓ done") {
		t.Errorf("expected '✓ done' in output, got: %q", out)
	}
	if !strings.Contains(out, "1 done") {
		t.Errorf("expected '1 done' in summary, got: %q", out)
	}

	var completed int
	env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id='t1'`).Scan(&completed)
	if completed != 1 {
		t.Error("expected task marked completed in local cache")
	}
}

func TestOverdue_SkipAction(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Write docs", "p1")
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "→ skipped") {
		t.Errorf("expected '→ skipped' in output, got: %q", out)
	}
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("expected '1 skipped' in summary, got: %q", out)
	}
}

func TestOverdue_RescheduleAction(t *testing.T) {
	var gotDueString string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && !strings.HasSuffix(r.URL.Path, "/close") {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if ds, ok := body["due_string"].(string); ok {
				gotDueString = ds
			}
		}
		writeJSON(w, map[string]string{"id": "t1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Review PR", "p1")
	setStdin(t, "r tomorrow\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if gotDueString != "tomorrow" {
		t.Errorf("expected due_string 'tomorrow' sent to API, got %q", gotDueString)
	}
	if !strings.Contains(out, "→ rescheduled") {
		t.Errorf("expected '→ rescheduled' in output, got: %q", out)
	}
	if !strings.Contains(out, "1 rescheduled") {
		t.Errorf("expected '1 rescheduled' in summary, got: %q", out)
	}
}

func TestOverdue_RescheduleInvalidDate_RepromptsAndRetries(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.Error(w, `{"error":"invalid date"}`, http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"id": "t1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Deploy service", "p1")
	setStdin(t, "r nonsense\nr tomorrow\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "invalid date") {
		t.Errorf("expected 'invalid date' error shown, got: %q", out)
	}
	if !strings.Contains(out, "→ rescheduled") {
		t.Errorf("expected successful reschedule after retry, got: %q", out)
	}
}

func TestOverdue_QuitAction(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Task one", "p1")
	seedOverdueTask(t, env, "t2", "Task two", "p1")
	setStdin(t, "q\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if strings.Contains(out, "Task two") {
		t.Errorf("expected quit to stop processing, but Task two appeared: %q", out)
	}
	if !strings.Contains(out, "0 done") {
		t.Errorf("expected '0 done' in summary, got: %q", out)
	}
}

func TestOverdue_MultipleTasksMixedActions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/close") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, map[string]string{"id": "t1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Task one", "p1")
	seedOverdueTask(t, env, "t2", "Task two", "p1")
	seedOverdueTask(t, env, "t3", "Task three", "p1")
	setStdin(t, "d\nr next monday\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "1 done") {
		t.Errorf("expected '1 done' in summary, got: %q", out)
	}
	if !strings.Contains(out, "1 rescheduled") {
		t.Errorf("expected '1 rescheduled' in summary, got: %q", out)
	}
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("expected '1 skipped' in summary, got: %q", out)
	}
}

func TestOverdue_InvalidInputShowsHelp(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Some task", "p1")
	setStdin(t, "x\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "unknown") {
		t.Errorf("expected 'unknown' hint on invalid input, got: %q", out)
	}
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("expected task skipped after invalid input, got: %q", out)
	}
}

func TestOverdue_ExcludesFutureTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('future', 'Future task', 'p1', date('now', '+1 day'))`)
	setStdin(t, "")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "no overdue tasks") {
		t.Errorf("expected future task excluded, got: %q", out)
	}
}

func TestOverdue_ExcludesTodayTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('today', 'Today task', 'p1', date('now'))`)
	setStdin(t, "")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "no overdue tasks") {
		t.Errorf("expected today's tasks excluded from overdue, got: %q", out)
	}
}
