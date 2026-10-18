package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// offlineAPI points the client at a port nothing listens on, so every request
// fails to connect, like a laptop without network.
func offlineAPI(t *testing.T) {
	t.Helper()
	t.Setenv("TODOIST_API_BASE", "http://127.0.0.1:1")
}

func seedOfflineTask(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Doing", "p1", 1)
	env.conn.Exec(`INSERT INTO tasks (id, content, description, project_id, section_id, priority, due_date, due_string, due_is_recurring, deadline_date, url, created_at)
		VALUES ('6hmROffline1', 'Prepare talk', 'Slides and demo', 'p1', 's1', 3, '2026-10-20', 'every monday', 1, '2026-11-01', 'https://app.todoist.com/app/task/6hmROffline1', '2026-10-01T09:00:00Z')`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('6hmROffline1', 'talks'), ('6hmROffline1', 'deep')`)
	hSeedSubtask(t, env.conn, "6hmROfflineS", "Draft outline", "p1", "6hmROffline1")
}

func TestShow_OfflineFallsBackToCache(t *testing.T) {
	env := newTestEnv(t, nil)
	offlineAPI(t)
	seedOfflineTask(t, env)

	var out string
	var err error
	stderr := captureStderr(t, func() { out, err = runCmd(t, "show", "6hmROffline1") })
	if err != nil {
		t.Fatalf("show offline: %v", err)
	}
	for _, want := range []string{"Prepare talk", "Slides and demo", "project   Work", "priority  3", "due       2026-10-20  (every monday)", "deadline  2026-11-01", "labels    deep, talks", "Draft outline", "https://app.todoist.com/app/task/6hmROffline1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in offline output:\n%s", want, out)
		}
	}
	if !strings.Contains(stderr, "offline") || !strings.Contains(stderr, "comments") {
		t.Errorf("stderr should explain the cached view and missing comments, got %q", stderr)
	}
}

func TestShow_OfflineJSON(t *testing.T) {
	env := newTestEnv(t, nil)
	offlineAPI(t)
	seedOfflineTask(t, env)

	var out string
	var err error
	captureStderr(t, func() { out, err = runCmd(t, "show", "6hmROffline1", "--json") })
	if err != nil {
		t.Fatalf("show --json offline: %v", err)
	}
	var got struct {
		ID       string   `json:"id"`
		Section  *string  `json:"section"`
		Labels   []string `json:"labels"`
		Deadline *string  `json:"deadline"`
		Due      *struct {
			Recurring bool `json:"recurring"`
		} `json:"due"`
		Subtasks []any `json:"subtasks"`
		Comments []any `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout must stay valid JSON offline: %v\n%s", err, out)
	}
	if got.ID != "6hmROffline1" || got.Section == nil || *got.Section != "Doing" || len(got.Labels) != 2 ||
		got.Deadline == nil || got.Due == nil || !got.Due.Recurring || len(got.Subtasks) != 1 || got.Comments == nil {
		t.Errorf("offline JSON = %+v", got)
	}
}

func TestShow_OfflineUncachedTaskReportsNetworkError(t *testing.T) {
	newTestEnv(t, nil)
	offlineAPI(t)
	if _, err := runCmd(t, "show", "notCached123"); err == nil {
		t.Error("expected the connection error for a task that is not cached")
	}
}

func TestShow_APIErrorDoesNotFallBack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Task not found","http_code":404}`))
	})
	env := newTestEnv(t, mux)
	seedOfflineTask(t, env)
	if _, err := runCmd(t, "show", "6hmROffline1"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v; a 404 means the task is gone and must not show stale cache", err)
	}
}
