package main

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func TestSections_RequiresProjectContext(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "sections")
	if err == nil {
		t.Fatal("expected error with no project context")
	}
}

func TestSections_ListsInOrder(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedSection(t, env.conn, "s2", "In Progress", "p1", 1)
	hSeedSection(t, env.conn, "s3", "Done", "p1", 2)

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "sections")
	if err != nil {
		t.Fatalf("sections: %v", err)
	}

	for _, name := range []string{"Backlog", "In Progress", "Done"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected section %q in output, got: %q", name, out)
		}
	}
	if !strings.Contains(out, "s1") {
		t.Errorf("expected section ID in output, got: %q", out)
	}

	backlogPos := strings.Index(out, "Backlog")
	progressPos := strings.Index(out, "In Progress")
	donePos := strings.Index(out, "Done")
	if !(backlogPos < progressPos && progressPos < donePos) {
		t.Errorf("sections out of order in output: %q", out)
	}
}

func TestSections_OnlyScopedToActiveProject(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s1", "Work Section", "p1", 0)
	hSeedSection(t, env.conn, "s2", "Personal Section", "p2", 0)

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "sections")
	if err != nil {
		t.Fatalf("sections: %v", err)
	}
	if !strings.Contains(out, "Work Section") {
		t.Errorf("expected 'Work Section', got: %q", out)
	}
	if strings.Contains(out, "Personal Section") {
		t.Errorf("'Personal Section' should not appear, got: %q", out)
	}
}

func TestSections_ExcludesArchived(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Active", "p1", 0)
	env.conn.Exec(`INSERT INTO sections (id, name, project_id, ord, is_archived) VALUES ('s2', 'Archived', 'p1', 1, 1)`)

	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "sections")
	if err != nil {
		t.Fatalf("sections: %v", err)
	}
	if strings.Contains(out, "Archived") {
		t.Errorf("archived section should not appear, got: %q", out)
	}
}
