package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// write478 emulates Todoist's "project not found" response: a 404 whose JSON
// body carries error_code 478 plus internal transport detail that must never
// reach the user.
func write478(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Project not found","error_code":478,` +
		`"error_extra":{"event_id":"E1","retry_after":3},` +
		`"error_tag":"NOT_FOUND","http_code":404}`))
}

// assertStaleProjectError checks the error was translated to the actionable
// message and that no raw transport detail leaked.
func assertStaleProjectError(t *testing.T, err error, project string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, project) {
		t.Errorf("expected project %q in error, got: %q", project, msg)
	}
	if !strings.Contains(msg, "todoist-cli sync") {
		t.Errorf("expected sync hint in error, got: %q", msg)
	}
	for _, leak := range []string{"event_id", "error_extra", "retry_after", "HTTP 404"} {
		if strings.Contains(msg, leak) {
			t.Errorf("raw transport detail %q leaked into error: %q", leak, msg)
		}
	}
}

func TestAdd_StaleProjectFlag_ActionableError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) { write478(w) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "arch", "Archive")

	_, err := runCmd(t, "add", "example task", "-p", "Archive")
	assertStaleProjectError(t, err, "Archive")
}

func TestAdd_StaleContext_ActionableError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) { write478(w) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "arch", "Archive")
	if err := state.Save(&state.State{ProjectID: "arch", ProjectName: "Archive"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	_, err := runCmd(t, "add", "example task")
	assertStaleProjectError(t, err, "Archive")
}

func TestMv_StaleProjectFlag_ActionableError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) { write478(w) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "arch", "Archive")
	hSeedTask(t, env.conn, "t1", "Task One", "p1", "")

	_, err := runCmd(t, "mv", "t1", "-p", "Archive")
	assertStaleProjectError(t, err, "Archive")
}

func TestEdit_StaleProjectFlag_ActionableError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) { write478(w) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "arch", "Archive")
	hSeedTask(t, env.conn, "t1", "Task One", "p1", "")

	_, err := runCmd(t, "edit", "t1", "-p", "Archive")
	assertStaleProjectError(t, err, "Archive")
}

func TestProjectsAdd_StaleParent_ActionableError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) { write478(w) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "arch", "Archive")

	_, err := runCmd(t, "projects", "add", "Child", "--parent", "Archive")
	assertStaleProjectError(t, err, "Archive")
}

// A non-478 API failure must pass through unchanged — never mislabeled as a
// stale-cache problem.
func TestAdd_NonStaleAPIError_PassesThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Content is empty","error_code":20,"error_tag":"BAD_REQUEST"}`))
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "arch", "Archive")

	_, err := runCmd(t, "add", "x", "-p", "Archive")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if strings.Contains(err.Error(), "todoist-cli sync") || strings.Contains(err.Error(), "cache may be stale") {
		t.Errorf("non-478 error must not be translated to the stale-cache message: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("expected raw API error for non-478 failure, got: %q", err.Error())
	}
}
