//go:build live

package main

import (
	"strings"
	"testing"
)

// TestLiveDoneByNameTwice: a task completed by name does not block a new task
// with the same name (#52).
func TestLiveDoneByNameTwice(t *testing.T) {
	token := liveEnv(t, "TODOIST_TOKEN_A")
	_, project := liveProject(t, token)
	mustRun(t, "sync")
	mustRun(t, "cd", project)

	mustRun(t, "add", "zeta")
	mustRun(t, "done", "zeta")
	again := strings.TrimSpace(mustRun(t, "add", "zeta"))
	if out := mustRun(t, "done", "zeta"); !strings.Contains(out, "zeta") {
		t.Errorf("done by name after an earlier done of the same name:\n%s", out)
	}
	if got := showTask(t, again); got.Content != "zeta" {
		t.Errorf("second zeta: %+v", got)
	}
}
