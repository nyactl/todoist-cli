//go:build live

package main

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// TestLiveOverdueReschedule: rescheduling in overdue keeps the new date in
// the cache (#49).
func TestLiveOverdueReschedule(t *testing.T) {
	token := liveEnv(t, "TODOIST_TOKEN_A")
	_, project := liveProject(t, token)
	mustRun(t, "sync")
	mustRun(t, "cd", project)

	late := strings.TrimSpace(mustRun(t, "add", "epsilon", "-D", "yesterday"))
	root.SetIn(strings.NewReader("r today\nq\n"))
	defer root.SetIn(nil)
	mustRun(t, "overdue")
	today := time.Now().Format("2006-01-02")
	listed := decodeJSON[[]jsonTask](t, mustRun(t, "ls", "--json"))
	if i := slices.IndexFunc(listed, func(j jsonTask) bool { return j.ID == late }); i < 0 || listed[i].Due == nil || listed[i].Due.Date != today {
		t.Errorf("rescheduled task should be due today in the cache: %+v", listed)
	}
}
