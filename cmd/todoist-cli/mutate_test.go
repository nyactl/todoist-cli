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
