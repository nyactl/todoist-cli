package db

import "testing"

func TestMigrationVersion_ValidName(t *testing.T) {
	v, err := migrationVersion("001_initial.sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1 {
		t.Errorf("got %d, want 1", v)
	}
}

func TestMigrationVersion_InvalidPrefix(t *testing.T) {
	_, err := migrationVersion("initial_schema.sql")
	if err == nil {
		t.Fatal("expected error for non-integer prefix, got nil")
	}
}
