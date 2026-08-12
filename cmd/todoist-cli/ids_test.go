package main

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func TestLs_IDs_PrependsFullID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "8a3fZZZ", "Buy gift", "p1", "")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--ids")
	if err != nil {
		t.Fatalf("ls --ids: %v", err)
	}
	if !strings.Contains(out, "8a3fZZZ") {
		t.Errorf("expected full ID '8a3fZZZ' prepended, got: %q", out)
	}
	if !strings.Contains(out, "Buy gift") {
		t.Errorf("expected task content in output, got: %q", out)
	}
}

func TestLs_NoIDs_OmitsID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "8a3fZZZ", "Buy gift", "p1", "")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(out, "8a3fZZZ") {
		t.Errorf("ID must not appear without --ids (default unchanged), got: %q", out)
	}
}

func TestSearch_IDs_PrependsFullID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "c12bZZZ", "findme task", "p1", "")

	out, err := runCmd(t, "search", "--ids", "findme")
	if err != nil {
		t.Fatalf("search --ids: %v", err)
	}
	if !strings.Contains(out, "c12bZZZ") {
		t.Errorf("expected full ID 'c12bZZZ' in search output, got: %q", out)
	}
}

func TestSearch_NoIDs_OmitsID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "c12bZZZ", "findme task", "p1", "")

	out, err := runCmd(t, "search", "findme")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if strings.Contains(out, "c12bZZZ") {
		t.Errorf("ID must not appear without --ids, got: %q", out)
	}
}

const completedIDItem = `{"task_id":"6hFmC3GH2PgP6pPX","content":"finished thing","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"}`

func TestLsDone_IDs_ShowsFullID(t *testing.T) {
	env := newTestEnv(t, completedHandler(completedIDItem))
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "week", "--ids")
	if err != nil {
		t.Fatalf("ls --done --ids: %v", err)
	}
	if !strings.Contains(out, "6hFmC3GH2PgP6pPX") {
		t.Errorf("expected the full completed task ID, got: %q", out)
	}
}

func TestLsDone_NoIDs_ShowsShortIDOnly(t *testing.T) {
	env := newTestEnv(t, completedHandler(completedIDItem))
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "week")
	if err != nil {
		t.Fatalf("ls --done: %v", err)
	}
	if strings.Contains(out, "6hFmC3GH2PgP6pPX") {
		t.Errorf("full ID must not appear without --ids, got: %q", out)
	}
	if !strings.Contains(out, "6hFm") {
		t.Errorf("expected the short ID prefix by default, got: %q", out)
	}
}
