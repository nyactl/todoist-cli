package main

import (
	"net/http"
	"strings"
	"testing"
)

// Tasks completed through the CLI stay cached until the next sync; they must
// not make an otherwise empty project look full.
func TestProjectsRm_IgnoresCompletedTasks(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		deleted = strings.TrimPrefix(r.URL.Path, "/projects/")
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "proj-done", "Finished")
	hSeedTask(t, env.conn, "t-done", "Last task", "proj-done", "")
	env.conn.Exec(`UPDATE tasks SET is_completed = 1 WHERE id = 't-done'`)

	if _, err := runCmd(t, "projects", "rm", "Finished"); err != nil {
		t.Fatalf("projects rm on a project with only completed tasks: %v", err)
	}
	if deleted != "proj-done" {
		t.Errorf("DELETE sent for %q, want proj-done", deleted)
	}
}
