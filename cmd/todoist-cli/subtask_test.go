package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestAdd_WithParent_SendsParentID(t *testing.T) {
	var gotReq todoist.CreateTaskRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotReq)
		writeJSON(w, todoist.Task{ID: "child-1", Content: "Subtask", ParentID: gotReq.ParentID, ProjectID: "p1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent-1", "Parent task", "p1", "")

	if _, err := runCmd(t, "add", "--parent", "Parent task", "Subtask"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if gotReq.ParentID != "parent-1" {
		t.Errorf("expected parent_id 'parent-1' sent to API, got %q", gotReq.ParentID)
	}
}

func TestAdd_WithParent_InheritsParentProject(t *testing.T) {
	var gotReq todoist.CreateTaskRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotReq)
		writeJSON(w, todoist.Task{ID: "child-1", Content: "Subtask", ParentID: "parent-1", ProjectID: "p1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent-1", "Parent task", "p1", "")

	// No --project flag — should inherit from parent.
	if _, err := runCmd(t, "add", "--parent", "parent-1", "Subtask"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if gotReq.ProjectID != "p1" {
		t.Errorf("expected project_id inherited from parent 'p1', got %q", gotReq.ProjectID)
	}
}

func TestAdd_WithParent_OutputsTaskID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, todoist.Task{ID: "child-1", Content: "Subtask", ParentID: "parent-1", ProjectID: "p1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent-1", "Parent task", "p1", "")

	out, err := runCmd(t, "add", "--parent", "parent-1", "Subtask")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "child-1") {
		t.Errorf("expected new task ID in output, got: %q", out)
	}
}

func TestAdd_WithParent_UnknownParent_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "add", "--parent", "nonexistent-task", "Subtask")
	if err == nil {
		t.Fatal("expected error for unknown parent task, got nil")
	}
}

func TestAdd_WithParent_AndSection_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent-1", "Parent task", "p1", "")
	hSeedSection(t, env.conn, "s1", "Backlog", "p1", 0)

	_, err := runCmd(t, "add", "--parent", "parent-1", "--section", "Backlog", "Subtask")
	if err == nil {
		t.Fatal("expected error when both --parent and --section are given, got nil")
	}
	if !strings.Contains(err.Error(), "--parent and --section cannot be used together") {
		t.Errorf("expected clear error message, got: %v", err)
	}
}

func TestAdd_WithParent_ResolvesParentByPrefix(t *testing.T) {
	var gotParentID string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req todoist.CreateTaskRequest
		json.NewDecoder(r.Body).Decode(&req)
		gotParentID = req.ParentID
		writeJSON(w, todoist.Task{ID: "child-1", Content: "Subtask", ParentID: req.ParentID, ProjectID: "p1"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "parent-full-id", "Epic task", "p1", "")

	// Resolve by ID prefix.
	if _, err := runCmd(t, "add", "--parent", "parent-f", "Subtask"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if gotParentID != "parent-full-id" {
		t.Errorf("expected parent resolved to 'parent-full-id', got %q", gotParentID)
	}
}

func TestAdd_WithParent_ExplicitProjectOverridesInherited(t *testing.T) {
	var gotReq todoist.CreateTaskRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotReq)
		writeJSON(w, todoist.Task{ID: "child-1", Content: "Subtask", ParentID: "parent-1", ProjectID: "p2"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "parent-1", "Parent task", "p1", "")

	if _, err := runCmd(t, "add", "--parent", "parent-1", "--project", "Personal", "Subtask"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if gotReq.ProjectID != "p2" {
		t.Errorf("expected explicit project 'p2' to override parent's project, got %q", gotReq.ProjectID)
	}
}
