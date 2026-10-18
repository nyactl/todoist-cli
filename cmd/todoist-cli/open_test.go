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

// reopenRecorder records the task ID in each POST /tasks/{id}/reopen.
func reopenRecorder(ids *[]string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/tasks/"), "/reopen"); ok {
			*ids = append(*ids, id)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestOpen_ResolvesShortIDOfCachedCompletedTask(t *testing.T) {
	var ids []string
	env := newTestEnv(t, reopenRecorder(&ids))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "6hmR8gq7XwPp2vJc", "Add issue templates", "p1", "")
	hSeedTask(t, env.conn, "6hmR8gj2QqLm9fRt", "Add badges", "p1", "")
	env.conn.Exec(`UPDATE tasks SET is_completed=1`)

	out, err := runCmd(t, "open", "6hmR8gq")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(ids) != 1 || ids[0] != "6hmR8gq7XwPp2vJc" {
		t.Errorf("reopened %v, want the full ID 6hmR8gq7XwPp2vJc", ids)
	}
	if !strings.Contains(out, "reopened  6hmR8gq") {
		t.Errorf("output = %q", out)
	}
	var done int
	env.conn.QueryRow(`SELECT is_completed FROM tasks WHERE id='6hmR8gq7XwPp2vJc'`).Scan(&done)
	if done != 0 {
		t.Error("cache still marks the task completed")
	}
}

func TestOpen_UncachedIDIsPassedThrough(t *testing.T) {
	var ids []string
	newTestEnv(t, reopenRecorder(&ids))
	if _, err := runCmd(t, "open", "9zzzNotInCache1"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(ids) != 1 || ids[0] != "9zzzNotInCache1" {
		t.Errorf("reopened %v", ids)
	}
}

func TestOpen_AmbiguousPrefixErrorsWithoutCallingAPI(t *testing.T) {
	var ids []string
	env := newTestEnv(t, reopenRecorder(&ids))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "6hmR8gq7XwPp2vJc", "A", "p1", "")
	hSeedTask(t, env.conn, "6hmR8gj2QqLm9fRt", "B", "p1", "")

	if _, err := runCmd(t, "open", "6hmR8g"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("err = %v, want ambiguous prefix", err)
	}
	if len(ids) != 0 {
		t.Errorf("API called with %v despite ambiguity", ids)
	}
}
