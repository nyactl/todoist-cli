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

func TestProjectsRm_NonEmpty_RefusesWithoutForce(t *testing.T) {
	var called bool
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "proj-full", "Work")
	hSeedTask(t, env.conn, "t1", "Ship it", "proj-full", "")

	_, err := runCmd(t, "projects", "rm", "Work")
	if err == nil {
		t.Fatal("expected error deleting non-empty project without --force, got nil")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected 'not empty' in error, got: %v", err)
	}
	if called {
		t.Error("expected no DELETE call to API when refusing non-empty project")
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM projects WHERE id = 'proj-full'`).Scan(&n)
	if n != 1 {
		t.Error("expected project to remain in cache when deletion is refused")
	}
}

func TestProjectsRm_NonEmpty_DeletesWithForce(t *testing.T) {
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
	hSeedProject(t, env.conn, "proj-force", "Work")
	hSeedTask(t, env.conn, "t1", "Ship it", "proj-force", "")

	out, err := runCmd(t, "projects", "rm", "Work", "--force")
	if err != nil {
		t.Fatalf("projects rm --force: %v", err)
	}
	if deletedID != "proj-force" {
		t.Errorf("expected DELETE called with 'proj-force', got %q", deletedID)
	}
	if !strings.Contains(out, "deleted: Work") {
		t.Errorf("expected 'deleted: Work' in output, got: %q", out)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM projects WHERE id = 'proj-force'`).Scan(&n)
	if n != 0 {
		t.Error("expected project removed from cache after forced delete")
	}
}

func TestProjectsRm_SubprojectTasks_RefusedEvenWhenTargetEmpty(t *testing.T) {
	var called bool
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	// Parent with zero direct tasks, but a sub-project full of them — the
	// exact silent-data-loss case from #11.
	hSeedProject(t, env.conn, "parent", "Dev")
	hSeedSubproject(t, env.conn, "child", "Immich", "parent")
	hSeedTask(t, env.conn, "t1", "task a", "child", "")
	hSeedTask(t, env.conn, "t2", "task b", "child", "")

	_, err := runCmd(t, "projects", "rm", "Dev")
	if err == nil {
		t.Fatal("expected refusal deleting a parent whose sub-project has tasks, got nil")
	}
	if !strings.Contains(err.Error(), "sub-project") {
		t.Errorf("expected error to mention sub-projects, got: %v", err)
	}
	if called {
		t.Error("expected no DELETE call to API when refusing cascade delete")
	}
	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM projects WHERE id = 'parent'`).Scan(&n)
	if n != 1 {
		t.Error("expected parent project to remain in cache when refused")
	}
}

func TestProjectsRm_EmptySubprojects_Allowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "parent", "Dev")
	hSeedSubproject(t, env.conn, "child", "Empty", "parent")

	if _, err := runCmd(t, "projects", "rm", "Dev"); err != nil {
		t.Fatalf("expected empty parent+sub-projects to delete without --force, got: %v", err)
	}
}

func TestProjectsRm_SubprojectTasks_DeletesWithForce(t *testing.T) {
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
	hSeedProject(t, env.conn, "parent", "Dev")
	hSeedSubproject(t, env.conn, "child", "Immich", "parent")
	hSeedTask(t, env.conn, "t1", "task a", "child", "")

	if _, err := runCmd(t, "projects", "rm", "Dev", "--force"); err != nil {
		t.Fatalf("projects rm --force: %v", err)
	}
	if deletedID != "parent" {
		t.Errorf("expected DELETE called with 'parent', got %q", deletedID)
	}
}

// --- projects mv (rename) ---

func TestProjectsMv_RenamesInAPIAndCache(t *testing.T) {
	var gotBody todoist.UpdateProjectRequest
	var gotID string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		gotID = strings.TrimPrefix(r.URL.Path, "/projects/")
		json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, todoist.Project{ID: gotID, Name: gotBody.Name})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "proj-ren", "Dev (new)")

	out, err := runCmd(t, "projects", "mv", "Dev (new)", "Dev")
	if err != nil {
		t.Fatalf("projects mv: %v", err)
	}
	if gotID != "proj-ren" {
		t.Errorf("expected POST to /projects/proj-ren, got %q", gotID)
	}
	if gotBody.Name != "Dev" {
		t.Errorf("expected new name 'Dev' sent to API, got %q", gotBody.Name)
	}
	if !strings.Contains(out, "renamed: Dev (new) -> Dev") {
		t.Errorf("expected rename confirmation in output, got: %q", out)
	}

	var cachedName string
	env.conn.QueryRowContext(context.Background(),
		`SELECT name FROM projects WHERE id = 'proj-ren'`).Scan(&cachedName)
	if cachedName != "Dev" {
		t.Errorf("expected cached name updated to 'Dev', got %q", cachedName)
	}
}

func TestProjectsMv_MultiWordNewName(t *testing.T) {
	var gotName string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		var body todoist.UpdateProjectRequest
		json.NewDecoder(r.Body).Decode(&body)
		gotName = body.Name
		writeJSON(w, todoist.Project{ID: "p-mw", Name: body.Name})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p-mw", "Old")

	if _, err := runCmd(t, "projects", "mv", "Old", "Reading", "List"); err != nil {
		t.Fatalf("projects mv: %v", err)
	}
	if gotName != "Reading List" {
		t.Errorf("expected joined new name 'Reading List', got %q", gotName)
	}
}

func TestProjectsMv_UnknownProject_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "projects", "mv", "NoSuchProject", "Whatever")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestProjectsMv_MissingNewName_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")

	_, err := runCmd(t, "projects", "mv", "Work")
	if err == nil {
		t.Fatal("expected error when new name is omitted, got nil")
	}
}

func TestProjectsMv_APIError_LeavesCacheUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p-err", "Original")

	_, err := runCmd(t, "projects", "mv", "Original", "Renamed")
	if err == nil {
		t.Fatal("expected error when API returns 400, got nil")
	}

	var name string
	env.conn.QueryRowContext(context.Background(),
		`SELECT name FROM projects WHERE id = 'p-err'`).Scan(&name)
	if name != "Original" {
		t.Errorf("expected cache name unchanged 'Original' on API failure, got %q", name)
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
