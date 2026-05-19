package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestSync_PopulatesDB(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{
		{ID: "l1", Name: "urgent"},
	}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{
		{ID: "p1", Name: "Work"},
	}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{
		{ID: "s1", Name: "Backlog", ProjectID: "p1"},
	}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{
		{ID: "t1", Content: "Buy milk", ProjectID: "p1", Priority: 1},
	}))
	env := newTestEnv(t, mux)

	out, err := runCmd(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !strings.Contains(out, "tasks") {
		t.Errorf("expected sync summary output, got: %q", out)
	}

	var content string
	err = env.conn.QueryRowContext(context.Background(),
		`SELECT content FROM tasks WHERE id = 't1'`).Scan(&content)
	if err != nil {
		t.Fatalf("task not found after sync: %v", err)
	}
	if content != "Buy milk" {
		t.Errorf("expected content 'Buy milk', got %q", content)
	}
}

func TestSync_PurgesTasksNotInAPI(t *testing.T) {
	env := newTestEnv(t, emptyAPI())
	// seed a task that the API will not return
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "stale-task", "Old task", "p1", "")

	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 'stale-task'`).Scan(&n)
	if n != 0 {
		t.Error("expected stale task to be purged after sync")
	}
}

func TestSync_UpdatesExistingTask(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{
		{ID: "p1", Name: "Work"},
	}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{
		{ID: "t1", Content: "Updated content", ProjectID: "p1", Priority: 1},
	}))
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Old content", "p1", "")

	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var content string
	env.conn.QueryRowContext(context.Background(),
		`SELECT content FROM tasks WHERE id = 't1'`).Scan(&content)
	if content != "Updated content" {
		t.Errorf("expected updated content, got %q", content)
	}
}

func TestSync_SectionOrderMatchesAPIOrder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{{ID: "p1", Name: "Work"}}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{
		{ID: "s1", Name: "Backlog", ProjectID: "p1", Order: 0},
		{ID: "s2", Name: "In Progress", ProjectID: "p1", Order: 0},
		{ID: "s3", Name: "Done", ProjectID: "p1", Order: 0},
	}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{}))
	env := newTestEnv(t, mux)

	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("sync: %v", err)
	}

	rows, err := env.conn.QueryContext(context.Background(),
		`SELECT name FROM sections WHERE project_id = 'p1' ORDER BY ord`)
	if err != nil {
		t.Fatalf("query sections: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}
	want := []string{"Backlog", "In Progress", "Done"}
	for i, w := range want {
		if i >= len(names) || names[i] != w {
			t.Errorf("section order wrong: got %v, want %v", names, want)
			break
		}
	}
}

// --- project-scoped sync tests ---

func projectSyncMux(projectID string, sections []todoist.Section, tasks []todoist.Task) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/sections", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project_id") == projectID {
			writeJSON(w, apiPage[todoist.Section]{Results: sections})
		} else {
			writeJSON(w, apiPage[todoist.Section]{Results: nil})
		}
	})
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project_id") == projectID {
			writeJSON(w, apiPage[todoist.Task]{Results: tasks})
		} else {
			writeJSON(w, apiPage[todoist.Task]{Results: nil})
		}
	})
	return mux
}

func TestSyncProject_OnlySyncsTargetProject(t *testing.T) {
	mux := projectSyncMux("p1",
		[]todoist.Section{{ID: "s1", Name: "Backlog", ProjectID: "p1"}},
		[]todoist.Task{{ID: "t1", Content: "Write tests", ProjectID: "p1", Priority: 1}},
	)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	// Task in p2 — must survive a p1-scoped sync.
	hSeedTask(t, env.conn, "t2", "Personal task", "p2", "")

	out, err := runCmd(t, "sync", "-p", "Work")
	if err != nil {
		t.Fatalf("sync -p: %v", err)
	}
	if !strings.Contains(out, "tasks") {
		t.Errorf("expected output, got: %q", out)
	}

	// p1 task synced
	var content string
	if err := env.conn.QueryRowContext(context.Background(),
		`SELECT content FROM tasks WHERE id = 't1'`).Scan(&content); err != nil {
		t.Fatalf("p1 task not found: %v", err)
	}

	// p2 task untouched
	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 't2'`).Scan(&n)
	if n != 1 {
		t.Error("project sync must not purge tasks from other projects")
	}
}

func TestSyncProject_PurgesStaleTasksInProject(t *testing.T) {
	// API returns t2 but not t1 — t1 should be purged from p1 only.
	mux := projectSyncMux("p1",
		nil,
		[]todoist.Task{{ID: "t2", Content: "Remaining", ProjectID: "p1", Priority: 1}},
	)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Stale", "p1", "")

	if _, err := runCmd(t, "sync", "-p", "Work"); err != nil {
		t.Fatalf("sync -p: %v", err)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 't1'`).Scan(&n)
	if n != 0 {
		t.Error("stale task in synced project should be purged")
	}
}

func TestSyncProject_UnknownProject_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "sync", "-p", "NoSuchProject")
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestSyncProject_ByID(t *testing.T) {
	mux := projectSyncMux("p1",
		nil,
		[]todoist.Task{{ID: "t1", Content: "Task", ProjectID: "p1", Priority: 1}},
	)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")

	if _, err := runCmd(t, "sync", "-p", "p1"); err != nil {
		t.Fatalf("sync -p by ID: %v", err)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 't1'`).Scan(&n)
	if n != 1 {
		t.Error("task should be synced when project specified by ID")
	}
}
