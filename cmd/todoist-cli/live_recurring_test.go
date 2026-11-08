//go:build live

package main

import (
	"strings"
	"testing"
	"time"
)

// TestLiveRecurringDone: closing a recurring task moves it to its next date
// and it stays listed (#47).
func TestLiveRecurringDone(t *testing.T) {
	token := liveEnv(t, "TODOIST_TOKEN_A")
	_, project := liveProject(t, token)
	mustRun(t, "sync")
	mustRun(t, "cd", project)

	daily := strings.TrimSpace(mustRun(t, "add", "delta", "-D", "every day"))
	if out := mustRun(t, "done", daily); !strings.Contains(out, "next: tomorrow") {
		t.Errorf("done on a daily task should report the next date:\n%s", out)
	}
	if out := mustRun(t, "ls", "--ids"); !strings.Contains(out, daily) {
		t.Errorf("daily task vanished from ls after done:\n%s", out)
	}
	if got := showTask(t, daily); got.Due == nil || got.Due.Date != time.Now().AddDate(0, 0, 1).Format("2006-01-02") {
		t.Errorf("daily task should be due tomorrow after done: %+v", got.Due)
	}
}
