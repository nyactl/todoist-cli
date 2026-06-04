package main

import (
	"context"
	"strings"
	"testing"
)

func TestSearch_MatchesContent(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Fix login bug", "p1", "")
	hSeedTask(t, env.conn, "t2", "Write tests", "p1", "")

	out, err := runCmd(t, "search", "login")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Fix login bug") {
		t.Errorf("expected matching task in output, got: %q", out)
	}
	if strings.Contains(out, "Write tests") {
		t.Errorf("expected non-matching task to be excluded, got: %q", out)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Deploy Service", "p1", "")

	out, err := runCmd(t, "search", "deploy")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Deploy Service") {
		t.Errorf("expected case-insensitive match, got: %q", out)
	}
}

func TestSearch_MatchesDescription(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, description) VALUES ('t1', 'Review PR', 'p1', 'check the database migration carefully')`)

	out, err := runCmd(t, "search", "migration")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Review PR") {
		t.Errorf("expected description match to surface the task, got: %q", out)
	}
}

func TestSearch_NoResults(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Write docs", "p1", "")

	out, err := runCmd(t, "search", "nonexistent")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("expected 'no results', got: %q", out)
	}
}

func TestSearch_MultipleProjects(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "t1", "Fix bug in API", "p1", "")
	hSeedTask(t, env.conn, "t2", "Fix bike", "p2", "")
	hSeedTask(t, env.conn, "t3", "Unrelated task", "p1", "")

	out, err := runCmd(t, "search", "fix")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Fix bug in API") {
		t.Errorf("expected Work task in results, got: %q", out)
	}
	if !strings.Contains(out, "Fix bike") {
		t.Errorf("expected Personal task in results, got: %q", out)
	}
	if strings.Contains(out, "Unrelated task") {
		t.Errorf("expected non-matching task excluded, got: %q", out)
	}
}

func TestSearch_MultiWordQuery(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Update user profile page", "p1", "")
	hSeedTask(t, env.conn, "t2", "Update settings", "p1", "")

	out, err := runCmd(t, "search", "user", "profile")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Update user profile page") {
		t.Errorf("expected multi-word match, got: %q", out)
	}
	if strings.Contains(out, "Update settings") {
		t.Errorf("expected non-matching task excluded, got: %q", out)
	}
}

func TestSearch_ExcludesCompletedTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Done task", "p1", "")
	env.conn.ExecContext(context.Background(),
		`UPDATE tasks SET is_completed=1 WHERE id='t1'`)

	out, err := runCmd(t, "search", "done")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("expected completed task to be excluded, got: %q", out)
	}
}

func TestSearch_ExcludesSubtasks(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent", "Parent task", "p1", "")
	hSeedSubtask(t, env.conn, "child", "Child subtask", "p1", "parent")

	out, err := runCmd(t, "search", "subtask")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("expected subtasks to be excluded from search, got: %q", out)
	}
}

func TestSearch_RequiresArg(t *testing.T) {
	newTestEnv(t, nil)
	_, err := runCmd(t, "search")
	if err == nil {
		t.Fatal("expected error when no query given, got nil")
	}
}
