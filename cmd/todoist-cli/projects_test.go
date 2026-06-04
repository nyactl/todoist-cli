package main

import (
	"strings"
	"testing"
)

func TestProjects_ListsAll(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProjectOrd(t, env.conn, "p1", "Work", 0)
	hSeedProjectOrd(t, env.conn, "p2", "Personal", 1)

	out, err := runCmd(t, "projects")
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if !strings.Contains(out, "p1\tWork") {
		t.Errorf("expected 'p1\\tWork' in output, got: %q", out)
	}
	if !strings.Contains(out, "p2\tPersonal") {
		t.Errorf("expected 'p2\\tPersonal' in output, got: %q", out)
	}
}

func TestProjects_EmptyDB(t *testing.T) {
	newTestEnv(t, nil)

	out, err := runCmd(t, "projects")
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for no projects, got: %q", out)
	}
}

func TestProjects_ExcludesArchived(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProjectOrd(t, env.conn, "p1", "Active", 0)
	hSeedArchivedProject(t, env.conn, "p2", "OldProject")

	out, err := runCmd(t, "projects")
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if !strings.Contains(out, "Active") {
		t.Errorf("expected active project in output, got: %q", out)
	}
	if strings.Contains(out, "OldProject") {
		t.Errorf("expected archived project to be excluded, got: %q", out)
	}
}

func TestProjects_OrderedByOrd(t *testing.T) {
	env := newTestEnv(t, nil)
	// Insert in reverse order — output must follow ord.
	hSeedProjectOrd(t, env.conn, "p3", "Side", 2)
	hSeedProjectOrd(t, env.conn, "p1", "Work", 0)
	hSeedProjectOrd(t, env.conn, "p2", "Personal", 1)

	out, err := runCmd(t, "projects")
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 projects, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "p1") {
		t.Errorf("expected Work (ord=0) first, got: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "p2") {
		t.Errorf("expected Personal (ord=1) second, got: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "p3") {
		t.Errorf("expected Side (ord=2) third, got: %q", lines[2])
	}
}

func TestProjects_OutputIsTabSeparated(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProjectOrd(t, env.conn, "px", "MyProject", 0)

	out, err := runCmd(t, "projects")
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if !strings.Contains(out, "\t") {
		t.Errorf("expected tab-separated output, got: %q", out)
	}
}
