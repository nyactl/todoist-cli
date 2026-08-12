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
