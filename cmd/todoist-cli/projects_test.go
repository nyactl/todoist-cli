package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
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

// --- projects add ---

func TestProjectsAdd_NoArgs_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "projects", "add")
	if err == nil {
		t.Fatal("expected error when no project name given, got nil")
	}
}

func TestProjectsAdd_CreatesProjectInAPIAndCache(t *testing.T) {
	var gotBody todoist.CreateProjectRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, todoist.Project{ID: "new-proj", Name: gotBody.Name})
	})
	env := newTestEnv(t, mux)

	out, err := runCmd(t, "projects", "add", "Health")
	if err != nil {
		t.Fatalf("projects add: %v", err)
	}
	if gotBody.Name != "Health" {
		t.Errorf("expected name 'Health' sent to API, got %q", gotBody.Name)
	}
	if !strings.Contains(out, "new-proj") {
		t.Errorf("expected project ID in output, got: %q", out)
	}

	var cachedName string
	env.conn.QueryRowContext(context.Background(),
		`SELECT name FROM projects WHERE id = 'new-proj'`).Scan(&cachedName)
	if cachedName != "Health" {
		t.Errorf("expected cached project name 'Health', got %q", cachedName)
	}
}

func TestProjectsAdd_MultiWordName(t *testing.T) {
	var gotName string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		var body todoist.CreateProjectRequest
		json.NewDecoder(r.Body).Decode(&body)
		gotName = body.Name
		writeJSON(w, todoist.Project{ID: "p-mw", Name: body.Name})
	})
	newTestEnv(t, mux)

	if _, err := runCmd(t, "projects", "add", "Side", "Projects"); err != nil {
		t.Fatalf("projects add: %v", err)
	}
	if gotName != "Side Projects" {
		t.Errorf("expected joined name 'Side Projects', got %q", gotName)
	}
}

func TestProjectsAdd_WithParent_SendsParentID(t *testing.T) {
	var gotBody todoist.CreateProjectRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, todoist.Project{ID: "child-proj", Name: gotBody.Name})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "parent-id", "Learning")

	if _, err := runCmd(t, "projects", "add", "Books", "--parent", "Learning"); err != nil {
		t.Fatalf("projects add --parent: %v", err)
	}
	if gotBody.ParentID != "parent-id" {
		t.Errorf("expected parent_id 'parent-id' sent to API, got %q", gotBody.ParentID)
	}
	if gotBody.Name != "Books" {
		t.Errorf("expected name 'Books' sent to API, got %q", gotBody.Name)
	}
}

func TestProjectsAdd_UnknownParent_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "projects", "add", "Child", "--parent", "NoSuchParent")
	if err == nil {
		t.Fatal("expected error for unknown parent project, got nil")
	}
}

func TestProjectsAdd_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	})
	newTestEnv(t, mux)

	_, err := runCmd(t, "projects", "add", "Broken")
	if err == nil {
		t.Fatal("expected error when API returns 400, got nil")
	}
}

// --- projects rm ---

func TestProjectsRm_DeletesFromAPIAndCache(t *testing.T) {
	var deletedID string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		deletedID = strings.TrimPrefix(r.URL.Path, "/projects/")
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "proj-del", "Finance")

	out, err := runCmd(t, "projects", "rm", "Finance")
	if err != nil {
		t.Fatalf("projects rm: %v", err)
	}
	if deletedID != "proj-del" {
		t.Errorf("expected DELETE called with 'proj-del', got %q", deletedID)
	}
	if !strings.Contains(out, "deleted: Finance") {
		t.Errorf("expected 'deleted: Finance' in rm output, got: %q", out)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM projects WHERE id = 'proj-del'`).Scan(&n)
	if n != 0 {
		t.Error("expected project to be removed from local cache after rm")
	}
}

func TestProjectsRm_UnknownProject_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "projects", "rm", "NoSuchProject")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestProjectsRm_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p-err", "Admin")

	_, err := runCmd(t, "projects", "rm", "Admin")
	if err == nil {
		t.Fatal("expected error when API returns 403, got nil")
	}

	// project must still be in local cache when API call fails
	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM projects WHERE id = 'p-err'`).Scan(&n)
	if n != 1 {
		t.Error("expected project to remain in cache when API delete fails")
	}
}
