package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// completedHandler stubs the completed-tasks endpoint with the given item JSON
// (empty string for no completions).
func completedHandler(itemJSON string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[` + itemJSON + `],"projects":{},"sections":{}}`))
	})
	return mux
}

func TestLsDone_EmptyWithContext_ShowsScopeHint(t *testing.T) {
	env := newTestEnv(t, completedHandler(""))
	hSeedProject(t, env.conn, "p1", "Japan 2026")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Japan 2026"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "today")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if !strings.Contains(out, "Japan 2026") {
		t.Errorf("expected the active project name in the hint, got: %q", out)
	}
	if !strings.Contains(out, "clear the context") {
		t.Errorf("expected a cue to clear the context, got: %q", out)
	}
}

func TestLsDone_EmptyNoContext_PlainMessage(t *testing.T) {
	newTestEnv(t, completedHandler(""))
	if err := state.Save(&state.State{}); err != nil {
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "today")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if strings.TrimSpace(out) != "nothing completed" {
		t.Errorf("expected plain 'nothing completed' with no context, got: %q", out)
	}
	if strings.Contains(out, "clear the context") {
		t.Errorf("hint must not appear without an active context, got: %q", out)
	}
}

func TestLsDone_Global_IgnoresContext(t *testing.T) {
	var gotProjectID string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		w.Write([]byte(`{"items":[],"projects":{},"sections":{}}`))
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "today", "--global")
	if err != nil {
		t.Fatalf("ls --done --global: %v", err)
	}
	if gotProjectID != "" {
		t.Errorf("expected --global to drop the project scope, got project_id %q", gotProjectID)
	}
	// With no context in play, the empty message must be the plain one.
	if strings.Contains(out, "clear the context") {
		t.Errorf("--global must not emit the context hint, got: %q", out)
	}
}

func TestLsDone_WithResults_ListsTasksNoHint(t *testing.T) {
	item := `{"task_id":"tc1","content":"finished thing","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"}`
	env := newTestEnv(t, completedHandler(item))
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "week")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if !strings.Contains(out, "finished thing") {
		t.Errorf("expected the completed task to be listed, got: %q", out)
	}
	if strings.Contains(out, "clear the context") {
		t.Errorf("hint must not appear when there are results, got: %q", out)
	}
}

func TestLsDone_ShortIDsAreUniqueAndAligned(t *testing.T) {
	items := `{"task_id":"6hmR8gX2qqHf9vPc","content":"Record demo","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"},` +
		`{"task_id":"6hmR8gXcRw4pJ6mf","content":"Add topics","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"},` +
		`{"task_id":"zz990000","content":"Other","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"}`
	env := newTestEnv(t, completedHandler(items))
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "today")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	for _, want := range []string{"✓  6hmR8gX2  Record demo", "✓  6hmR8gXc  Add topics", "✓  zz99      Other"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func completedTwoProjects() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[` +
			`{"task_id":"c1","content":"Ship release","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"},` +
			`{"task_id":"c2","content":"Mow lawn","project_id":"p2","completed_at":"2026-08-12T10:00:00Z"},` +
			`{"task_id":"c3","content":"Orphan","project_id":"gone","completed_at":"2026-08-12T11:00:00Z"}` +
			`],"projects":{"p1":{"name":"Work"},"p2":{"name":"Home"}},"sections":{}}`))
	})
	return mux
}

func TestLsDone_NoContext_GroupsByProject(t *testing.T) {
	newTestEnv(t, completedTwoProjects())
	out, err := runCmd(t, "ls", "--done", "week")
	if err != nil {
		t.Fatal(err)
	}
	work, home, orphan := strings.Index(out, "  Work\n"), strings.Index(out, "  Home\n"), strings.Index(out, "(no project)")
	if work < 0 || home < 0 || orphan < 0 || !(work < home && home < orphan) {
		t.Fatalf("expected Work, Home, (no project) groups in API order:\n%s", out)
	}
	if !strings.Contains(out[work:home], "Ship release") || !strings.Contains(out[home:orphan], "Mow lawn") {
		t.Errorf("tasks under the wrong project:\n%s", out)
	}
}

func TestLsDone_LabelFilters(t *testing.T) {
	env := newTestEnv(t, completedTwoProjects())
	for _, row := range [][2]string{{"c1", "release"}, {"c1", "work"}, {"c2", "work"}} {
		env.conn.Exec(`INSERT INTO tasks (id, content, is_completed) VALUES (?, 'x', 1) ON CONFLICT DO NOTHING`, row[0])
		env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES (?, ?)`, row[0], row[1])
	}

	out, err := runCmd(t, "ls", "--done", "week", "-l", "work", "-l", "release")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Ship release") || strings.Contains(out, "Mow lawn") || strings.Contains(out, "Orphan") {
		t.Errorf("-l work -l release should keep only c1:\n%s", out)
	}

	out, err = runCmd(t, "ls", "--done", "week", "--not-label", "release")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Ship release") || !strings.Contains(out, "Mow lawn") || !strings.Contains(out, "Orphan") {
		t.Errorf("--not-label release should drop only c1:\n%s", out)
	}
}

func TestLsDone_APIErrorIsReturned(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	newTestEnv(t, mux)
	if _, err := runCmd(t, "ls", "--done", "today"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("err = %v, want the HTTP 502", err)
	}
}
