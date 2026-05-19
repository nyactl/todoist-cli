package main

import (
	"strings"
	"testing"
)

func TestCp_CopiesURL(t *testing.T) {
	if !clipboardAvailable() {
		t.Skip("no clipboard available in this environment")
	}

	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Buy milk", "p1", "")

	out, err := runCmd(t, "cp", "Buy milk")
	if err != nil {
		t.Fatalf("cp: %v", err)
	}
	if !strings.Contains(out, "https://app.todoist.com/app/task/task1") {
		t.Errorf("expected constructed URL in output, got: %q", out)
	}
}

func TestCp_SubstringMatch(t *testing.T) {
	if !clipboardAvailable() {
		t.Skip("no clipboard available in this environment")
	}

	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Lampen kaufen", "p1", "")

	out, err := runCmd(t, "cp", "Lampen")
	if err != nil {
		t.Fatalf("cp with substring: %v", err)
	}
	if !strings.Contains(out, "https://app.todoist.com/app/task/task1") {
		t.Errorf("expected URL in output, got: %q", out)
	}
}

func TestCp_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "cp", "no-such-task")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestCp_URLContainsTaskID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "6WvQ4gWpvvCG8f35", "bend", "p1", "")

	task, err := taskByIDForTest(t, env, "6WvQ4gWpvvCG8f35")
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	want := "https://app.todoist.com/app/task/6WvQ4gWpvvCG8f35"
	if got := taskURL(task.ID); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
