package main

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func TestLs_NotLabel_ExcludesInContext(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "actionable task", "p1", "")
	hSeedTask(t, env.conn, "t2", "parked task", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2','someday')`)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--not-label", "someday")
	if err != nil {
		t.Fatalf("ls --not-label: %v", err)
	}
	if !strings.Contains(out, "actionable task") {
		t.Errorf("expected non-excluded task shown, got: %q", out)
	}
	if strings.Contains(out, "parked task") {
		t.Errorf("someday task must be excluded, got: %q", out)
	}
}

func TestLs_LabelAndNotLabel_Combined(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "work only", "p1", "")
	hSeedTask(t, env.conn, "t2", "work parked", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1','work')`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2','work'),('t2','someday')`)

	out, err := runCmd(t, "ls", "-l", "work", "--not-label", "someday")
	if err != nil {
		t.Fatalf("ls -l --not-label: %v", err)
	}
	if !strings.Contains(out, "work only") {
		t.Errorf("expected included non-excluded task, got: %q", out)
	}
	if strings.Contains(out, "work parked") {
		t.Errorf("task with excluded label must be hidden, got: %q", out)
	}
}

func TestLs_UnknownLabel_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "task", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1','real')`)

	cases := [][]string{
		{"ls", "-l", "typo"},
		{"ls", "--not-label", "typo"},
		{"ls", "-l", "!someday"}, // plausible negation syntax — now self-diagnosing
	}
	for _, args := range cases {
		_, err := runCmd(t, args...)
		if err == nil {
			t.Fatalf("%v: expected unknown-label error, got nil", args)
		}
		if !strings.Contains(err.Error(), "unknown label") {
			t.Errorf("%v: expected 'unknown label' error, got: %v", args, err)
		}
	}
}

func TestLs_KnownLabelNoTasks_NotError(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "later", 0) // personal label, on no tasks

	out, err := runCmd(t, "ls", "-l", "later")
	if err != nil {
		t.Fatalf("a known label with no tasks must not error: %v", err)
	}
	if !strings.Contains(out, "no tasks") {
		t.Errorf("expected 'no tasks', got: %q", out)
	}
}

func TestLsDone_NotLabel_Excludes(t *testing.T) {
	items := `{"task_id":"tc1","content":"parked done","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"},` +
		`{"task_id":"tc2","content":"real done","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"}`
	env := newTestEnv(t, completedHandler(items))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "tc1", "parked done", "p1", "")
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('tc1','someday')`)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "ls", "--done", "week", "--not-label", "someday")
	if err != nil {
		t.Fatalf("ls --done --not-label: %v", err)
	}
	if strings.Contains(out, "parked done") {
		t.Errorf("someday completed task must be excluded, got: %q", out)
	}
	if !strings.Contains(out, "real done") {
		t.Errorf("non-excluded completed task should show, got: %q", out)
	}
}
