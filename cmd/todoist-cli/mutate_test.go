package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
	"github.com/nyactl/todoist-cli/internal/todoist"
)

// --- add ---

func TestAdd_CreatesTaskInActiveProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, todoist.Task{ID: "new-task", Content: "Buy coffee", ProjectID: "p1", Priority: 1})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	out, err := runCmd(t, "add", "Buy coffee")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "new-task") {
		t.Errorf("expected task ID in output, got: %q", out)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 'new-task'`).Scan(&n)
	if n != 1 {
		t.Error("expected task to be in local cache after add")
	}
}

func TestAdd_WithSection_SendsCorrectSectionID(t *testing.T) {
	var gotSectionID string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req todoist.CreateTaskRequest
		json.NewDecoder(r.Body).Decode(&req)
		gotSectionID = req.SectionID
		writeJSON(w, todoist.Task{ID: "task2", Content: "Plan sprint", ProjectID: "p1", SectionID: req.SectionID, Priority: 1})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "add", "--section", "Backlog", "Plan sprint"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if gotSectionID != "s1" {
		t.Errorf("expected section_id 's1' sent to API, got %q", gotSectionID)
	}
}

func TestAdd_WithPriority_SendsCorrectPriority(t *testing.T) {
	var gotPriority int
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req todoist.CreateTaskRequest
		json.NewDecoder(r.Body).Decode(&req)
		gotPriority = req.Priority
		writeJSON(w, todoist.Task{ID: "t1", Content: "Urgent task", Priority: req.Priority})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "add", "Urgent task", "--priority", "4"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if gotPriority != 4 {
		t.Errorf("expected priority 4 sent to API, got %d", gotPriority)
	}
}

func TestAdd_WithPriority_StoredInCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req todoist.CreateTaskRequest
		json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, todoist.Task{ID: "t-prio", Content: "High priority", Priority: req.Priority})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "add", "High priority", "--priority", "3"); err != nil {
		t.Fatalf("add: %v", err)
	}

	var priority int
	env.conn.QueryRowContext(context.Background(),
		`SELECT priority FROM tasks WHERE id = 't-prio'`).Scan(&priority)
	if priority != 3 {
		t.Errorf("expected priority 3 in local cache, got %d", priority)
	}
}

func TestAdd_WithoutPriority_OmitsFieldFromRequest(t *testing.T) {
	var body map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, todoist.Task{ID: "t2", Content: "Normal task", Priority: 1})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"})

	if _, err := runCmd(t, "add", "Normal task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, ok := body["priority"]; ok {
		t.Error("expected priority field to be absent from request when not specified")
	}
}

func TestAdd_InvalidPriority_Errors(t *testing.T) {
	newTestEnv(t, nil)
	_, err := runCmd(t, "add", "Task", "--priority", "5")
	if err == nil {
		t.Fatal("expected error for priority 5, got nil")
	}
}

func TestAdd_NegativePriority_Errors(t *testing.T) {
	newTestEnv(t, nil)
	_, err := runCmd(t, "add", "Task", "--priority=-1")
	if err == nil {
		t.Fatal("expected error for negative priority, got nil")
	}
}

func TestAdd_WithoutProject_ErrorWhenSectionGiven(t *testing.T) {
	env := newTestEnv(t, emptyAPI())
	hSeedProject(t, env.conn, "p1", "Work")
	// no context set

	_, err := runCmd(t, "add", "--section", "Backlog", "Task")
	if err == nil {
		t.Fatal("expected error when --section used without project context")
	}
}

// --- done ---

func TestDone_MarksTaskComplete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		// handles POST /tasks/{id}/close
		noopHandler(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Fix bug", "p1", "")

	if _, err := runCmd(t, "done", "Fix bug"); err != nil {
		t.Fatalf("done: %v", err)
	}

	var completed int
	env.conn.QueryRowContext(context.Background(),
		`SELECT is_completed FROM tasks WHERE id = 'task-abc'`).Scan(&completed)
	if completed != 1 {
		t.Error("expected task to be marked completed in local cache")
	}
}

func TestDone_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "done", "no-such-task")
	if err == nil {
		t.Fatal("expected error for unknown task, got nil")
	}
}

// --- mv ---

func TestMv_UpdatesSectionInDB(t *testing.T) {
	var movedToSection string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/move") {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			movedToSection = body["section_id"]
		}
		noopHandler(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedSection(t, env.conn, "s2", "In Progress", "p1", 1)
	hSeedTask(t, env.conn, "task1", "Write tests", "p1", "s1")
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("set context: %v", err)
	}

	if _, err := runCmd(t, "mv", "Write tests", "In Progress"); err != nil {
		t.Fatalf("mv: %v", err)
	}

	var sectionID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT section_id FROM tasks WHERE id = 'task1'`).Scan(&sectionID)
	if sectionID != "s2" {
		t.Errorf("expected section_id 's2' after mv, got %q", sectionID)
	}
	if movedToSection != "s2" {
		t.Errorf("expected API called with section_id 's2', got %q", movedToSection)
	}
}

func TestMv_RequiresProjectContext(t *testing.T) {
	newTestEnv(t, nil)
	// no context set

	_, err := runCmd(t, "mv", "some task", "some section")
	if err == nil {
		t.Fatal("expected error when no project context set")
	}
}

func TestMv_NeitherSectionNorProject_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Write tests", "p1", "")

	_, err := runCmd(t, "mv", "Write tests")
	if err == nil {
		t.Fatal("expected error when no section or -p given, got nil")
	}
}

func TestMv_BothPositionalAndProject_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedTask(t, env.conn, "t1", "Write tests", "p1", "")

	_, err := runCmd(t, "mv", "Write tests", "Backlog", "-p", "Personal")
	if err == nil {
		t.Fatal("expected error when both positional section and -p given, got nil")
	}
}

func TestMv_ToProject_CallsAPIWithProjectID(t *testing.T) {
	var gotBody map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/move") {
			json.NewDecoder(r.Body).Decode(&gotBody)
		}
		noopHandler(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "task1", "Buy coffee", "p1", "")

	if _, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal"); err != nil {
		t.Fatalf("mv -p: %v", err)
	}
	if gotBody["project_id"] != "p2" {
		t.Errorf("expected project_id 'p2' sent to API, got %q", gotBody["project_id"])
	}
}

func TestMv_ToProject_UpdatesCacheAndClearsSection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", noopHandler)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)
	hSeedTask(t, env.conn, "task1", "Buy coffee", "p1", "s1")

	out, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal")
	if err != nil {
		t.Fatalf("mv -p: %v", err)
	}
	if !strings.Contains(out, "Personal") {
		t.Errorf("expected project name in output, got: %q", out)
	}

	var projectID, sectionID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT project_id, COALESCE(section_id, '') FROM tasks WHERE id = 'task1'`).
		Scan(&projectID, &sectionID)
	if projectID != "p2" {
		t.Errorf("expected project_id 'p2' in cache, got %q", projectID)
	}
	if sectionID != "" {
		t.Errorf("expected section_id cleared after cross-project move, got %q", sectionID)
	}
}

func TestMv_ToProjectWithSection_SendsSectionID(t *testing.T) {
	var gotBody map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/move") {
			json.NewDecoder(r.Body).Decode(&gotBody)
		}
		noopHandler(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s2", "Later", "p2", 0)
	hSeedTask(t, env.conn, "task1", "Buy coffee", "p1", "")

	if _, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal", "-s", "Later"); err != nil {
		t.Fatalf("mv -p -s: %v", err)
	}
	if gotBody["section_id"] != "s2" {
		t.Errorf("expected section_id 's2' sent to API, got %q", gotBody["section_id"])
	}
	if gotBody["project_id"] != "" {
		t.Errorf("expected no project_id in request when section_id provided, got %q", gotBody["project_id"])
	}
}

func TestMv_ToProjectWithSection_UpdatesCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", noopHandler)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedSection(t, env.conn, "s2", "Later", "p2", 0)
	hSeedTask(t, env.conn, "task1", "Buy coffee", "p1", "")

	out, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal", "-s", "Later")
	if err != nil {
		t.Fatalf("mv -p -s: %v", err)
	}
	if !strings.Contains(out, "Personal") || !strings.Contains(out, "Later") {
		t.Errorf("expected project and section in output, got: %q", out)
	}

	var projectID, sectionID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT project_id, COALESCE(section_id, '') FROM tasks WHERE id = 'task1'`).
		Scan(&projectID, &sectionID)
	if projectID != "p2" {
		t.Errorf("expected project_id 'p2' in cache, got %q", projectID)
	}
	if sectionID != "s2" {
		t.Errorf("expected section_id 's2' in cache, got %q", sectionID)
	}
}

func TestMv_ToProject_UnknownProject_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Buy coffee", "p1", "")

	_, err := runCmd(t, "mv", "Buy coffee", "-p", "NoSuchProject")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestMv_ToProjectWithSection_UnknownSection_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "t1", "Buy coffee", "p1", "")

	_, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal", "-s", "NoSuchSection")
	if err == nil {
		t.Fatal("expected error for unknown section in destination project, got nil")
	}
}

func TestMv_ToProject_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "task1", "Buy coffee", "p1", "")

	_, err := runCmd(t, "mv", "Buy coffee", "-p", "Personal")
	if err == nil {
		t.Fatal("expected error when API returns 403, got nil")
	}

	// cache must be unchanged
	var projectID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT project_id FROM tasks WHERE id = 'task1'`).Scan(&projectID)
	if projectID != "p1" {
		t.Errorf("expected task to remain in original project after API failure, got project_id %q", projectID)
	}
}

// --- rm ---

func TestRm_DeletesTaskFromDB(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", noopHandler)
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-del", "Delete me", "p1", "")

	out, err := runCmd(t, "rm", "Delete me")
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if !strings.Contains(out, "Delete me") {
		t.Errorf("expected task name in rm output, got: %q", out)
	}

	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM tasks WHERE id = 'task-del'`).Scan(&n)
	if n != 0 {
		t.Error("expected task to be removed from local cache after rm")
	}
}

func TestRm_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "rm", "ghost-task")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}
