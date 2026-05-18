package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func makeEditStub(t *testing.T, taskID string, onUpdate func(map[string]any), returnTask todoist.Task) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/"+taskID+"/move", noopHandler)
	mux.HandleFunc("/tasks/"+taskID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if onUpdate != nil {
				onUpdate(body)
			}
			writeJSON(w, returnTask)
			return
		}
		http.NotFound(w, r)
	})
	return mux
}

func TestEdit_UpdatesContent(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "New content", Priority: 1})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Old content", "p1", "")

	out, err := runCmd(t, "edit", "Old content", "--content", "New content")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if sentFields["content"] != "New content" {
		t.Errorf("expected content sent to API, got: %v", sentFields)
	}
	if !strings.Contains(out, "New content") {
		t.Errorf("expected updated content in output, got: %q", out)
	}

	var content string
	env.conn.QueryRowContext(context.Background(),
		`SELECT content FROM tasks WHERE id = 'task1'`).Scan(&content)
	if content != "New content" {
		t.Errorf("expected cache updated to 'New content', got %q", content)
	}
}

func TestEdit_UpdatesPriority(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "Task", Priority: 3})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	if _, err := runCmd(t, "edit", "Task", "-P", "3"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if int(sentFields["priority"].(float64)) != 3 {
		t.Errorf("expected priority 3 sent to API, got: %v", sentFields)
	}
}

func TestEdit_InvalidPriority_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	_, err := runCmd(t, "edit", "Task", "-P", "5")
	if err == nil {
		t.Fatal("expected error for priority out of range")
	}
}

func TestEdit_SetsDueDate(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "Task", Priority: 1,
			Due: &todoist.Due{Date: "2026-05-20", String: "next tuesday"}})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	if _, err := runCmd(t, "edit", "Task", "-D", "next tuesday"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if sentFields["due_string"] != "next tuesday" {
		t.Errorf("expected due_string sent to API, got: %v", sentFields)
	}
}

func TestEdit_ClearsDueDate(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "Task", Priority: 1})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('task1', 'Task', 'p1', '2026-05-20')`)

	if _, err := runCmd(t, "edit", "Task", "-D", ""); err != nil {
		t.Fatalf("edit: %v", err)
	}
	// Empty string must be explicitly sent — not omitted
	if v, ok := sentFields["due_string"]; !ok || v != "" {
		t.Errorf("expected due_string: '' sent to API, got: %v", sentFields)
	}
}

func TestEdit_UpdatesLabels(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "Task", Priority: 1, Labels: []string{"urgent"}})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	if _, err := runCmd(t, "edit", "Task", "-l", "urgent"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	labels, ok := sentFields["labels"].([]any)
	if !ok || len(labels) != 1 || labels[0] != "urgent" {
		t.Errorf("expected labels [urgent] sent to API, got: %v", sentFields)
	}
}

func TestEdit_MovesToProject(t *testing.T) {
	var movedToProject string
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/task1/move", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		movedToProject = body["project_id"]
		noopHandler(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Personal")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	if _, err := runCmd(t, "edit", "Task", "-p", "Personal"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if movedToProject != "p2" {
		t.Errorf("expected project_id 'p2' sent to API, got %q", movedToProject)
	}

	var projectID string
	env.conn.QueryRowContext(context.Background(),
		`SELECT project_id FROM tasks WHERE id = 'task1'`).Scan(&projectID)
	if projectID != "p2" {
		t.Errorf("expected cache updated to project p2, got %q", projectID)
	}
}

func TestEdit_MultipleFlags(t *testing.T) {
	var sentFields map[string]any
	stub := makeEditStub(t, "task1", func(f map[string]any) { sentFields = f },
		todoist.Task{ID: "task1", Content: "New title", Priority: 2})

	env := newTestEnv(t, stub)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Old title", "p1", "")

	if _, err := runCmd(t, "edit", "Old title", "-c", "New title", "-P", "2"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if sentFields["content"] != "New title" {
		t.Errorf("expected content in fields, got: %v", sentFields)
	}
	if int(sentFields["priority"].(float64)) != 2 {
		t.Errorf("expected priority in fields, got: %v", sentFields)
	}
}

func TestEdit_NoFlags_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task1", "Task", "p1", "")

	_, err := runCmd(t, "edit", "Task")
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
}

func TestEdit_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "edit", "no-such-task", "-c", "new")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}
