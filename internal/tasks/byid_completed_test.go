package tasks_test

import (
	"context"
	"testing"

	"github.com/nyactl/todoist-cli/internal/tasks"
)

// A task completed through the CLI stays cached until the next sync; it must
// not make the title of a new active task ambiguous.
func TestByID_NameIgnoresCompletedTasks(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Home")
	seedTask(t, conn, "old11111", "Buy milk", "p1", "")
	seedTask(t, conn, "new22222", "Buy milk", "p1", "")
	conn.Exec(`UPDATE tasks SET is_completed = 1 WHERE id = 'old11111'`)

	for _, q := range []string{"Buy milk", "milk"} {
		got, err := tasks.ByID(context.Background(), conn, q)
		if err != nil {
			t.Fatalf("ByID(%q): %v", q, err)
		}
		if got.ID != "new22222" {
			t.Errorf("ByID(%q) = %s, want the active task", q, got.ID)
		}
	}
}

// IDs and prefixes still reach completed tasks, which open relies on.
func TestByID_IDStillFindsCompletedTask(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Home")
	seedTask(t, conn, "old11111", "Buy milk", "p1", "")
	conn.Exec(`UPDATE tasks SET is_completed = 1 WHERE id = 'old11111'`)

	for _, q := range []string{"old11111", "old1"} {
		if got, err := tasks.ByID(context.Background(), conn, q); err != nil || got.ID != "old11111" {
			t.Errorf("ByID(%q) = %v, %v", q, got, err)
		}
	}
}
