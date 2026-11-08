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
