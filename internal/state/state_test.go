package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func withTempDataDir(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func TestState_RoundTrip(t *testing.T) {
	withTempDataDir(t)

	want := &state.State{ProjectID: "proj1", ProjectName: "Work"}
	if err := state.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := state.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ProjectID != want.ProjectID || got.ProjectName != want.ProjectName {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestState_Clear(t *testing.T) {
	withTempDataDir(t)

	if err := state.Save(&state.State{ProjectID: "proj1", ProjectName: "Work"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := state.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	got, err := state.Load()
	if err != nil {
		t.Fatalf("Load after clear: %v", err)
	}
	if got.HasProject() {
		t.Errorf("expected empty state after clear, got %+v", got)
	}
}

func TestState_MissingFile(t *testing.T) {
	withTempDataDir(t)

	got, err := state.Load()
	if err != nil {
		t.Fatalf("Load with no file: %v", err)
	}
	if got.HasProject() {
		t.Errorf("expected empty state, got %+v", got)
	}
}

func TestState_HasProject(t *testing.T) {
	if (&state.State{ProjectID: "x"}).HasProject() != true {
		t.Error("HasProject should be true when ProjectID is set")
	}
	if (&state.State{}).HasProject() != false {
		t.Error("HasProject should be false when ProjectID is empty")
	}
	var s *state.State
	if s.HasProject() != false {
		t.Error("HasProject should be false on nil state")
	}
}

func TestState_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	// Write invalid JSON — Load should return empty state, not an error.
	path := filepath.Join(dir, "todoist-cli", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := state.Load()
	if err != nil {
		t.Fatalf("Load with corrupted JSON should not error, got: %v", err)
	}
	if got.HasProject() {
		t.Errorf("expected empty state for corrupted JSON, got %+v", got)
	}
}

func TestState_ClearWhenNoFile(t *testing.T) {
	withTempDataDir(t)

	// Clear with no state file should return an error (os.Remove on missing file).
	err := state.Clear()
	if err == nil {
		t.Fatal("expected error when clearing non-existent state file")
	}
}

func TestState_FallbackToHomeDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", t.TempDir())

	want := &state.State{ProjectID: "proj2", ProjectName: "Personal"}
	if err := state.Save(want); err != nil {
		t.Fatalf("Save without XDG_DATA_HOME: %v", err)
	}

	got, err := state.Load()
	if err != nil {
		t.Fatalf("Load without XDG_DATA_HOME: %v", err)
	}
	if got.ProjectID != want.ProjectID {
		t.Errorf("got %q, want %q", got.ProjectID, want.ProjectID)
	}
}
