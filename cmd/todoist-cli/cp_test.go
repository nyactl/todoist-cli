package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestCp_CopiesURL(t *testing.T) {
	if !clipboardAvailable() {
		t.Skip("no clipboard available in this environment")
	}

	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, url) VALUES ('task1', 'Buy milk', 'p1', 'https://todoist.com/app/task/task1')`)

	out, err := runCmd(t, "cp", "Buy milk")
	if err != nil {
		t.Fatalf("cp: %v", err)
	}
	if !strings.Contains(out, "https://todoist.com/app/task/task1") {
		t.Errorf("expected URL in output, got: %q", out)
	}
}

func TestCp_NoURL_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Buy milk", "p1", "")

	_, err := runCmd(t, "cp", "Buy milk")
	if err == nil {
		t.Fatal("expected error when task has no URL")
	}
}

func TestCp_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "cp", "no-such-task")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestCp_SubstringMatch(t *testing.T) {
	if !clipboardAvailable() {
		t.Skip("no clipboard available in this environment")
	}

	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, url) VALUES ('task1', 'Lampen kaufen', 'p1', 'https://todoist.com/app/task/task1')`)

	out, err := runCmd(t, "cp", "Lampen")
	if err != nil {
		t.Fatalf("cp with substring: %v", err)
	}
	if !strings.Contains(out, "https://todoist.com/app/task/task1") {
		t.Errorf("expected URL in output, got: %q", out)
	}
}

func TestCp_URLFallbackFromAPI(t *testing.T) {
	if !clipboardAvailable() {
		t.Skip("no clipboard available in this environment")
	}

	liveURL := "https://todoist.com/app/task/task1"
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/task1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"id":      "task1",
			"content": "Buy bend wire",
			"url":     liveURL,
		})
	})

	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	// Insert task with no URL — simulates stale cache
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id) VALUES ('task1', 'Buy bend wire', 'p1')`)

	out, err := runCmd(t, "cp", "bend")
	if err != nil {
		t.Fatalf("cp URL fallback: %v", err)
	}
	if !strings.Contains(out, liveURL) {
		t.Errorf("expected live URL in output, got: %q", out)
	}
}

func TestCp_URLStoredInTask(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, url) VALUES ('task1', 'Write report', 'p1', 'https://todoist.com/app/task/task1')`)

	// Verify URL is accessible via tasks.ByID
	task, err := taskByIDForTest(t, env, "task1")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if task.URL != "https://todoist.com/app/task/task1" {
		t.Errorf("expected URL in task, got %q", task.URL)
	}
}
