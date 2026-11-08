package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// An unreadable state file makes loadContext return a nil state; completion
// for open must not panic on it.
func TestComplete_OpenSurvivesUnreadableState(t *testing.T) {
	newTestEnv(t, http.NewServeMux())
	stateDir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "todoist-cli", "state.json")
	if err := os.MkdirAll(stateDir, 0o700); err != nil { // a directory cannot be read as a file
		t.Fatal(err)
	}
	if _, dir := complete(t, "open", ""); dir == "" {
		t.Error("expected a completion directive")
	}
}

// A comment with a short or missing posted_at must not crash the details view.
func TestOverdue_ViewSurvivesShortCommentTimestamp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{Results: []todoist.Comment{
			{ID: "c1", TaskID: "t1", Content: "No timestamp", PostedAt: ""},
		}})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Deploy service", "p1")
	setStdin(t, "v\ns\n")

	out, err := runCmd(t, "overdue")
	if err != nil {
		t.Fatalf("overdue: %v", err)
	}
	if !strings.Contains(out, "No timestamp") {
		t.Errorf("comment missing from details:\n%s", out)
	}
}
