package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
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

// emptyComments returns a handler stub that responds to GET /comments with an empty list.
// Use this when the test presses v but doesn't care about comment output.
func emptyComments() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", pageResponse([]todoist.Comment{}))
	return mux
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

// --- header: project / section / labels (#31) ---

func TestOverdue_Header_ShowsProject(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Personal")
	seedOverdueTask(t, env, "t1", "Buy groceries", "p1")
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Personal") {
		t.Errorf("expected project name in header, got: %q", out)
	}
}

func TestOverdue_Header_ShowsProjectAndSection(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, section_id, due_date) VALUES ('t1', 'Fix login bug', 'p1', 's1', date('now', '-3 days'))`)
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Work / Backlog") {
		t.Errorf("expected 'Work / Backlog' in header, got: %q", out)
	}
}

func TestOverdue_Header_ShowsLabels(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Review PR", "p1")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "urgent") {
		t.Errorf("expected label 'urgent' in header, got: %q", out)
	}
}

func TestOverdue_Header_ProjectAndLabelsSeparatedByDot(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Review PR", "p1")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Work  ·  urgent") {
		t.Errorf("expected 'Work  ·  urgent' format in header, got: %q", out)
	}
}

func TestOverdue_Header_MultipleLabels_CommaSeparated(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Review PR", "p1")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent'), ('t1', 'backend')`)
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "urgent") || !strings.Contains(out, "backend") {
		t.Errorf("expected both labels in header, got: %q", out)
	}
	// labels must be comma-separated, not run together
	if !strings.Contains(out, "urgent, backend") && !strings.Contains(out, "backend, urgent") {
		t.Errorf("expected labels comma-separated in header, got: %q", out)
	}
}

func TestOverdue_Header_ProjectSectionAndLabels_Combined(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, section_id, due_date) VALUES ('t1', 'Fix bug', 'p1', 's1', date('now', '-3 days'))`)
	env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Work / Backlog  ·  urgent") {
		t.Errorf("expected 'Work / Backlog  ·  urgent' in header, got: %q", out)
	}
}

func TestOverdue_Header_NoLabels_OmitsDotSeparator(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Some task", "p1")
	setStdin(t, "s\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	// "·" appears in the summary line ("0 done · 0 rescheduled") — check the
	// header-specific form instead.
	if strings.Contains(out, "Work  ·") {
		t.Errorf("expected no dot separator in header when no labels present, got: %q", out)
	}
}

// --- v = details: description + comments (#32) ---

func TestOverdue_ViewAction_ShowsDescription(t *testing.T) {
	env := newTestEnv(t, emptyComments())
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date, description) VALUES ('t1', 'Fix login bug', 'p1', date('now', '-3 days'), 'Check the auth middleware first')`)
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Check the auth middleware first") {
		t.Errorf("expected description in v output, got: %q", out)
	}
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("expected task skipped after viewing, got: %q", out)
	}
}

func TestOverdue_ViewAction_ShowsComments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{Results: []todoist.Comment{
			{ID: "c1", TaskID: "t1", Content: "Confirmed in staging", PostedAt: "2026-06-01T10:00:00Z"},
			{ID: "c2", TaskID: "t1", Content: "Blocked on deploy window", PostedAt: "2026-06-03T14:00:00Z"},
		}})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Deploy service", "p1")
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "comments") {
		t.Errorf("expected 'comments' section header in v output, got: %q", out)
	}
	if !strings.Contains(out, "Confirmed in staging") {
		t.Errorf("expected first comment in v output, got: %q", out)
	}
	if !strings.Contains(out, "Blocked on deploy window") {
		t.Errorf("expected second comment in v output, got: %q", out)
	}
	if !strings.Contains(out, "2026-06-01") {
		t.Errorf("expected comment date in v output, got: %q", out)
	}
}

func TestOverdue_ViewAction_NoComments_OmitsSection(t *testing.T) {
	env := newTestEnv(t, emptyComments())
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Some task", "p1")
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if strings.Contains(out, "comments") {
		t.Errorf("expected no 'comments' section when there are none, got: %q", out)
	}
}

func TestOverdue_ViewAction_CommentsAPIError_StillShowsDescription(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date, description) VALUES ('t1', 'Fix bug', 'p1', date('now', '-3 days'), 'See the ticket for context')`)
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "See the ticket for context") {
		t.Errorf("expected description shown even when comments API fails, got: %q", out)
	}
	if strings.Contains(out, "comments") {
		t.Errorf("expected comments section omitted on API error, got: %q", out)
	}
}

func TestOverdue_ViewAction_ShowsURL(t *testing.T) {
	env := newTestEnv(t, emptyComments())
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date, url) VALUES ('t1', 'Fix bug', 'p1', date('now', '-3 days'), 'https://todoist.com/task/t1')`)
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "https://todoist.com/task/t1") {
		t.Errorf("expected URL in v output, got: %q", out)
	}
}

func TestOverdue_ViewAction_DescriptionAndComments_BothShown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{Results: []todoist.Comment{
			{ID: "c1", TaskID: "t1", Content: "Blocked on infra", PostedAt: "2026-06-02T09:00:00Z"},
		}})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date, description) VALUES ('t1', 'Deploy service', 'p1', date('now', '-3 days'), 'Run the migration first')`)
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "Run the migration first") {
		t.Errorf("expected description in v output, got: %q", out)
	}
	if !strings.Contains(out, "Blocked on infra") {
		t.Errorf("expected comment in v output, got: %q", out)
	}
}

func TestOverdue_ViewAction_CommentsRequestUsesCorrectTaskID(t *testing.T) {
	var gotTaskID string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		gotTaskID = r.URL.Query().Get("task_id")
		writeJSON(w, apiPage[todoist.Comment]{Results: []todoist.Comment{}})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "task-abc123", "Deploy service", "p1")
	setStdin(t, "v\ns\n")

	if _, err := runCmd(t, "overdue"); err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if gotTaskID != "task-abc123" {
		t.Errorf("expected comments fetched with task_id 'task-abc123', got %q", gotTaskID)
	}
}

func TestOverdue_ViewAction_DoesNotRepeatProjectInV(t *testing.T) {
	env := newTestEnv(t, emptyComments())
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Some task", "p1")
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	// "Work" appears once in the header; count occurrences to confirm v doesn't repeat it
	if count := strings.Count(out, "Work"); count != 1 {
		t.Errorf("expected 'Work' exactly once (in header, not repeated in v), got %d occurrences: %q", count, out)
	}
}

func TestOverdue_ViewAction_DoesNotAdvance(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/close") {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/comments", pageResponse([]todoist.Comment{}))
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Fix login bug", "p1")
	seedOverdueTask(t, env, "t2", "Write tests", "p1")
	// v on t1, then d on t1 (not t2), then s on t2
	setStdin(t, "v\nd\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "1 done") {
		t.Errorf("expected '1 done' in summary, got: %q", out)
	}
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("expected '1 skipped' in summary, got: %q", out)
	}

	var completed int
	env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id='t1'`).Scan(&completed)
	if completed != 1 {
		t.Error("expected t1 (not t2) to be marked completed after v then d")
	}
	env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id='t2'`).Scan(&completed)
	if completed != 0 {
		t.Error("expected t2 to remain incomplete (only skipped)")
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
