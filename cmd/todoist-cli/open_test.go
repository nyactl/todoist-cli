package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestOpen_ReopensTask(t *testing.T) {
	var reopenCalled bool
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/reopen") {
			reopenCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-done", "Completed task", "p1", "")
	env.conn.ExecContext(context.Background(),
		`UPDATE tasks SET is_completed=1 WHERE id='task-done'`)

	_, err := runCmd(t, "open", "task-done")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !reopenCalled {
		t.Error("expected POST /tasks/{id}/reopen to be called")
	}
}

func TestOpen_UpdatesDBToNotCompleted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-done2", "Another done task", "p1", "")
	env.conn.ExecContext(context.Background(),
		`UPDATE tasks SET is_completed=1 WHERE id='task-done2'`)

	if _, err := runCmd(t, "open", "task-done2"); err != nil {
		t.Fatalf("open: %v", err)
	}

	var completed int
	env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id='task-done2'`).Scan(&completed)
	if completed != 0 {
		t.Error("expected task to be marked not completed in local cache after open")
	}
}

func TestOpen_PrintsReopenedID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-reopen", "Task to reopen", "p1", "")

	out, err := runCmd(t, "open", "task-reopen")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.Contains(out, "reopened") {
		t.Errorf("expected 'reopened' in output, got: %q", out)
	}
}

func TestOpen_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	newTestEnv(t, mux)

	_, err := runCmd(t, "open", "nonexistent-task-id")
	if err == nil {
		t.Fatal("expected error when API returns 404, got nil")
	}
}
