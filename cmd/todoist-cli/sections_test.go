package main

import (
	"context"
	"net/http"
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

// --- sections rm ---

func TestSectionsRm_DeletesFromAPIAndCache(t *testing.T) {
	var deletedID string
	mux := http.NewServeMux()
	mux.HandleFunc("/sections/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		deletedID = strings.TrimPrefix(r.URL.Path, "/sections/")
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "sections", "rm", "Backlog")
	if err != nil {
		t.Fatalf("sections rm: %v", err)
	}
	if deletedID != "s1" {
		t.Errorf("expected DELETE called with 's1', got %q", deletedID)
	}
	if !strings.Contains(out, "deleted: Backlog") {
		t.Errorf("expected 'deleted: Backlog' in output, got: %q", out)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sections WHERE id = 's1'`).Scan(&n)
	if n != 0 {
		t.Error("expected section removed from local cache after rm")
	}
}

func TestSectionsRm_RequiresProjectContext(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "sections", "rm", "Backlog")
	if err == nil {
		t.Fatal("expected error with no project context")
	}
}

func TestSectionsRm_UnknownSection_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	_, err := runCmd(t, "sections", "rm", "NoSuchSection")
	if err == nil {
		t.Fatal("expected error for unknown section, got nil")
	}
}

func TestSectionsRm_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sections/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	_, err := runCmd(t, "sections", "rm", "Backlog")
	if err == nil {
		t.Fatal("expected error when API returns 403, got nil")
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sections WHERE id = 's1'`).Scan(&n)
	if n != 1 {
		t.Error("expected section to remain in cache when API delete fails")
	}
}

func TestSectionsRm_TasksSectionClearedOnDelete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sections/", noopHandler)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedTask(t, env.conn, "t1", "Write tests", "p1", "s1")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "sections", "rm", "Backlog"); err != nil {
		t.Fatalf("sections rm: %v", err)
	}

	var sectionID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT COALESCE(section_id, '') FROM tasks WHERE id = 't1'`).Scan(&sectionID)
	if sectionID != "" {
		t.Errorf("expected task section_id cleared after section deleted, got %q", sectionID)
	}
}

func TestSectionsRm_OnlyDeletesFromActiveProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sections/", noopHandler)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedSection(t, env.conn, "s2", "Backlog", "p2", 0)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "sections", "rm", "Backlog"); err != nil {
		t.Fatalf("sections rm: %v", err)
	}

	// s1 (Work) deleted, s2 (Personal) untouched
	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sections WHERE id = 's1'`).Scan(&n)
	if n != 0 {
		t.Error("expected s1 removed from cache")
	}
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sections WHERE id = 's2'`).Scan(&n)
	if n != 1 {
		t.Error("expected s2 (different project, same name) to remain in cache")
	}
}
