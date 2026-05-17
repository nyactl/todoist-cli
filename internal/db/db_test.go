package db_test

import (
	"path/filepath"
	"testing"

	"github.com/nyactl/todoist-cli/internal/db"
)

func TestOpenAt_MigratesCleanly(t *testing.T) {
	conn, err := db.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer conn.Close()

	var version int
	if err := conn.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("schema_migrations query: %v", err)
	}
	if version < 1 {
		t.Fatalf("expected at least migration version 1, got %d", version)
	}
}

func TestOpenAt_TablesExist(t *testing.T) {
	conn, err := db.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer conn.Close()

	for _, table := range []string{"projects", "sections", "tasks", "task_labels", "labels", "sync_state"} {
		var n int
		err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
		if err != nil || n == 0 {
			t.Errorf("table %q missing", table)
		}
	}
}

func TestOpenAt_IdempotentMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	conn, err := db.OpenAt(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	conn.Close()

	conn2, err := db.OpenAt(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	conn2.Close()
}
