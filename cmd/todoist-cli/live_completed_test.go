//go:build live

package main

import (
	"strings"
	"testing"
)

// TestLiveCompletedLabels: the label filter on completed tasks works after a
// sync has dropped them from the cache, labels stay out of the content, and
// a year spans several API windows (#45).
func TestLiveCompletedLabels(t *testing.T) {
	token := liveEnv(t, "TODOIST_TOKEN_A")
	_, project := liveProject(t, token)
	label := liveName("label")
	mustRun(t, "sync")
	mustRun(t, "cd", project)

	mustRun(t, "done", strings.TrimSpace(mustRun(t, "add", "labelled done", "-l", label)))
	mustRun(t, "sync")
	out := mustRun(t, "ls", "--done", "today", "-l", label)
	if !strings.Contains(out, "labelled done") || strings.Contains(out, "@") {
		t.Errorf("ls --done -l after sync:\n%s", out)
	}
	if out := mustRun(t, "ls", "--done", "year"); !strings.Contains(out, "labelled done") {
		t.Errorf("ls --done year should span several API windows and include today:\n%s", out)
	}
}
