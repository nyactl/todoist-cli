package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// seedTwoProjects sets up Inbox(inbox) and Work(w1) > Backend(w2), with an
// active cd context on Work.
func seedTwoProjects(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "inbox", "Inbox")
	hSeedProject(t, env.conn, "w1", "Work")
	hSeedSubproject(t, env.conn, "w2", "Backend", "w1")
	hSeedTask(t, env.conn, "t-in", "inbox task", "inbox", "")
	hSeedTask(t, env.conn, "t-w1", "work task", "w1", "")
	hSeedTask(t, env.conn, "t-w2", "backend task", "w2", "")
	if err := state.Save(&state.State{ProjectID: "w1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}
}

func assertContextUnchanged(t *testing.T) {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.ProjectID != "w1" || st.ProjectName != "Work" {
		t.Errorf("ls -p must not touch the stored context, got %+v", st)
	}
}

func TestLs_Project_ListsThatProjectOnly(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)

	out, err := runCmd(t, "ls", "-p", "Inbox")
	if err != nil {
		t.Fatalf("ls -p: %v", err)
	}
	if !strings.Contains(out, "inbox task") {
		t.Errorf("expected the Inbox task, got: %q", out)
	}
	if strings.Contains(out, "work task") || strings.Contains(out, "backend task") {
		t.Errorf("tasks of the active context must not appear, got: %q", out)
	}
	assertContextUnchanged(t)
}

func TestLs_Project_JSON(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)

	out, err := runCmd(t, "ls", "--project", "Inbox", "--json")
	if err != nil {
		t.Fatalf("ls -p --json: %v", err)
	}
	if !strings.Contains(out, `"inbox task"`) || strings.Contains(out, "work task") {
		t.Errorf("expected only the Inbox task in JSON, got: %q", out)
	}
	assertContextUnchanged(t)
}

func TestLs_Project_ByID(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)

	out, err := runCmd(t, "ls", "-p", "inbox")
	if err != nil {
		t.Fatalf("ls -p <id>: %v", err)
	}
	if !strings.Contains(out, "inbox task") {
		t.Errorf("expected the Inbox task, got: %q", out)
	}
}

func TestLs_Project_WithoutContext(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	if err := state.Clear(); err != nil {
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "-p", "Work")
	if err != nil {
		t.Fatalf("ls -p: %v", err)
	}
	// Undated tasks only show in a project view, not in the today agenda.
	if !strings.Contains(out, "work task") {
		t.Errorf("expected the project view of Work, got: %q", out)
	}
	st, _ := state.Load()
	if st.HasProject() {
		t.Errorf("ls -p must not set a context, got %+v", st)
	}
}

func TestLs_Project_Recursive(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	if err := state.Clear(); err != nil {
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "-p", "Work", "-r")
	if err != nil {
		t.Fatalf("ls -p -r: %v", err)
	}
	if !strings.Contains(out, "work task") || !strings.Contains(out, "backend task") {
		t.Errorf("expected Work and its sub-project, got: %q", out)
	}
	if strings.Contains(out, "inbox task") {
		t.Errorf("unrelated project leaked into -r, got: %q", out)
	}
}

func TestLs_Project_Label(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	hSeedLabel(t, env.conn, "l1", "urgent", 1)
	if _, err := env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t-in','urgent'),('t-w1','urgent'),('t-w2','urgent')`); err != nil {
		t.Fatalf("label tasks: %v", err)
	}

	out, err := runCmd(t, "ls", "-p", "Inbox", "-l", "urgent")
	if err != nil {
		t.Fatalf("ls -p -l: %v", err)
	}
	if !strings.Contains(out, "inbox task") || strings.Contains(out, "work task") {
		t.Errorf("expected the label query scoped to Inbox, got: %q", out)
	}
}

func TestLs_Project_Board(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	if err := state.Clear(); err != nil {
		t.Fatalf("clear context: %v", err)
	}

	out, err := runCmd(t, "ls", "-p", "Inbox", "-b")
	if err != nil {
		t.Fatalf("ls -p -b: %v", err)
	}
	if !strings.Contains(out, "inbox task") {
		t.Errorf("expected the board of Inbox, got: %q", out)
	}
}

func TestLs_Project_WithGlobal_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)

	_, err := runCmd(t, "ls", "-p", "Inbox", "-g")
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("expected -p and -g to be rejected, got %v", err)
	}
}

func TestLs_Project_Unknown_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)

	_, err := runCmd(t, "ls", "-p", "Nope")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected a not-found error, got %v", err)
	}
	assertContextUnchanged(t)
}

func TestLs_Project_Ambiguous_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	hSeedProject(t, env.conn, "inbox2", "Inbox")

	_, err := runCmd(t, "ls", "-p", "Inbox")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected an ambiguity error, got %v", err)
	}
}

func TestLs_Project_Empty_NoContextHint(t *testing.T) {
	env := newTestEnv(t, nil)
	seedTwoProjects(t, env)
	hSeedProject(t, env.conn, "e1", "Empty")

	out, err := runCmd(t, "ls", "-p", "e1")
	if err != nil {
		t.Fatalf("ls -p: %v", err)
	}
	if !strings.Contains(out, `"Empty"`) {
		t.Errorf("expected the project's name (not its ID) in the empty message, got: %q", out)
	}
	if strings.Contains(out, "clear the context") {
		t.Errorf("-p is not a context; the cd hint would mislead, got: %q", out)
	}
}

func TestLsDone_Project_ScopesAPICall(t *testing.T) {
	var gotProjectID string
	mux := http.NewServeMux()
	mux.HandleFunc(completedPath, func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		w.Write([]byte(`{"items":[],"next_cursor":null}`))
	})
	env := newTestEnv(t, mux)
	seedTwoProjects(t, env)

	out, err := runCmd(t, "ls", "--done", "today", "-p", "Inbox")
	if err != nil {
		t.Fatalf("ls --done -p: %v", err)
	}
	if gotProjectID != "inbox" {
		t.Errorf("expected project_id=inbox, got %q", gotProjectID)
	}
	if strings.Contains(out, "clear the context") {
		t.Errorf("-p is not a context; the cd hint would mislead, got: %q", out)
	}
	assertContextUnchanged(t)
}
