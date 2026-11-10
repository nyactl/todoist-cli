package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nyactl/todoist-cli/internal/db"
)

// Caches written before timed due dates were split hold the full value in
// due_date; migration 004 splits them so date queries see these tasks again
// without a sync.
func TestMigration004_SplitsTimedDueDates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO tasks (id, content, due_date) VALUES
		('floating', 'a', '2026-11-09T15:00:00'),
		('fixed',    'b', '2026-11-09T22:00:00.000000Z'),
		('allday',   'c', '2026-11-09')`); err != nil {
		t.Fatal(err)
	}
	// Pretend the cache predates migration 004.
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE version >= 4`); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	conn, err = db.OpenAt(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer conn.Close()

	// A UTC value lands on the local day; run under several TZ values.
	fixedLocal := time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC).Local().Format("2006-01-02")
	want := map[string][2]string{
		"floating": {"2026-11-09", "2026-11-09T15:00:00"},
		"fixed":    {fixedLocal, "2026-11-09T22:00:00.000000Z"},
		"allday":   {"2026-11-09", ""},
	}
	for id, w := range want {
		var date, datetime string
		if err := conn.QueryRow(`SELECT due_date, COALESCE(due_datetime, '') FROM tasks WHERE id = ?`, id).Scan(&date, &datetime); err != nil {
			t.Fatal(err)
		}
		if date != w[0] || datetime != w[1] {
			t.Errorf("%s: due_date=%q due_datetime=%q, want %q %q", id, date, datetime, w[0], w[1])
		}
	}
}
