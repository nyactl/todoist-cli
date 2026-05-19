package main

import (
	"os/exec"
	"strings"
	"testing"
)

// fakeEmptyPath returns a temp dir with no binaries — used to simulate fzf not installed.
func fakeEmptyPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func TestPick_FzfNotInstalled(t *testing.T) {
	newTestEnv(t, nil)
	fakeEmptyPath(t)

	_, err := runCmd(t, "pick")
	if err == nil {
		t.Fatal("expected error when fzf is not installed")
	}
	if !strings.Contains(err.Error(), "fzf not found") {
		t.Errorf("expected 'fzf not found' error, got: %v", err)
	}
}

func TestPick_NoTasks_NoContext(t *testing.T) {
	newTestEnv(t, nil)
	fakeEmptyPath(t)

	// No tasks seeded — but fzf-not-found fires first. We want to test the no-tasks
	// branch, so we need fzf on PATH. Skip if it's not available in this environment.
	if _, err := exec.LookPath("fzf"); err != nil {
		// fzf unavailable: verify the fzf-not-found error is consistent
		_, cmdErr := runCmd(t, "pick")
		if cmdErr == nil || !strings.Contains(cmdErr.Error(), "fzf not found") {
			t.Errorf("expected fzf-not-found error, got: %v", cmdErr)
		}
		return
	}

	// fzf available but no tasks — pick should exit cleanly (no tasks path).
	// We can't drive fzf interactively in tests, so this is a best-effort check.
	t.Skip("fzf available but interactive pick cannot be driven in tests")
}

func TestPick_FzfNotFound_WithTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	fakeEmptyPath(t)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Write tests", "p1", "")

	_, err := runCmd(t, "pick")
	if err == nil {
		t.Fatal("expected error when fzf not installed")
	}
	if !strings.Contains(err.Error(), "fzf not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPick_LabelFlag_FzfNotInstalled(t *testing.T) {
	env := newTestEnv(t, nil)
	fakeEmptyPath(t)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Urgent task", "p1", "")

	_, err := runCmd(t, "pick", "-l", "urgent")
	if err == nil {
		t.Fatal("expected error when fzf not installed")
	}
	if !strings.Contains(err.Error(), "fzf not found") {
		t.Errorf("unexpected error: %v", err)
	}
}
