package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// A task rescheduled to today must still be listed by ls without a sync.
func TestOverdue_RescheduleKeepsNewDueDateInCache(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, todoist.Task{ID: "t1", Content: "Review PR", ProjectID: "p1", Priority: 1,
			Due: &todoist.Due{Date: today, String: "today"}})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	seedOverdueTask(t, env, "t1", "Review PR", "p1")
	setStdin(t, "r today\n")

	if _, err := runCmd(t, "overdue"); err != nil {
		t.Fatalf("overdue: %v", err)
	}
	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Review PR") || !strings.Contains(out, "today") {
		t.Errorf("task rescheduled to today missing from ls:\n%s", out)
	}
}
