package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// recurringAPI closes any task and then serves it again active with its due
// date advanced, like Todoist does for a recurring task.
func recurringAPI(t *testing.T, closed *[]string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/tasks/")
		if id, ok := strings.CutSuffix(id, "/close"); ok {
			*closed = append(*closed, id)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, todoist.Task{ID: id, Content: "Water plants", ProjectID: "p1", Priority: 1,
			Due: &todoist.Due{Date: "2030-01-02", String: "every day", IsRecurring: true}})
	})
	return mux
}

func seedRecurring(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Home")
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date, due_string, due_is_recurring)
		VALUES ('rec00001', 'Water plants', 'p1', date('now', 'localtime', '-1 day'), 'every day', 1)`)
}

func assertStillActive(t *testing.T, env *testEnv) {
	t.Helper()
	var completed int
	var due string
	if err := env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed, due_date FROM tasks WHERE id = 'rec00001'`).Scan(&completed, &due); err != nil {
		t.Fatal(err)
	}
	if completed != 0 || due != "2030-01-02" {
		t.Errorf("recurring task after close: is_completed=%d due=%s, want 0 and the next date 2030-01-02", completed, due)
	}
}

func TestDone_RecurringTaskStaysActiveWithNextDate(t *testing.T) {
	var closed []string
	env := newTestEnv(t, recurringAPI(t, &closed))
	seedRecurring(t, env)

	if _, err := runCmd(t, "done", "rec00001"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if len(closed) != 1 {
		t.Fatalf("close calls = %v", closed)
	}
	assertStillActive(t, env)
}

func TestOverdue_DoneOnRecurringTaskStaysActive(t *testing.T) {
	var closed []string
	env := newTestEnv(t, recurringAPI(t, &closed))
	seedRecurring(t, env)

	root.SetIn(strings.NewReader("d\n"))
	defer root.SetIn(nil)
	if _, err := runCmd(t, "overdue"); err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if len(closed) != 1 {
		t.Fatalf("close calls = %v", closed)
	}
	assertStillActive(t, env)
}

func taskState(t *testing.T, env *testEnv, id string) (completed int) {
	t.Helper()
	if err := env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id = ?`, id).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	return completed
}

// The last occurrence of a recurring task really completes it.
func TestDone_RecurringLastOccurrenceIsCompleted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/close") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, todoist.Task{ID: "rec00001", Content: "Water plants", ProjectID: "p1", Priority: 1, IsCompleted: true,
			Due: &todoist.Due{Date: "2026-11-01", String: "every day until nov 1", IsRecurring: true}})
	})
	env := newTestEnv(t, mux)
	seedRecurring(t, env)

	out, err := runCmd(t, "done", "rec00001")
	if err != nil {
		t.Fatalf("done: %v", err)
	}
	if taskState(t, env, "rec00001") != 1 {
		t.Error("a recurring task whose series ended must be marked complete")
	}
	if strings.Contains(out, "next:") {
		t.Errorf("no next date after the last occurrence:\n%s", out)
	}
}

// A one-off task is marked complete without fetching it again.
func TestDone_OneOffTaskSkipsRefetch(t *testing.T) {
	var gets int
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		}
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Home")
	hSeedTask(t, env.conn, "one00001", "Buy milk", "p1", "")

	if _, err := runCmd(t, "done", "one00001"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if taskState(t, env, "one00001") != 1 || gets != 0 {
		t.Errorf("one-off task: is_completed=%d, GET calls=%d; want 1 and 0", taskState(t, env, "one00001"), gets)
	}
}
