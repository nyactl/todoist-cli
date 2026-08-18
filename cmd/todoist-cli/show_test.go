package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestFormatCommentTime(t *testing.T) {
	shape := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
	for _, in := range []string{
		"2026-08-18T05:47:50.325458Z", // fractional seconds (real API shape)
		"2026-06-01T09:00:00Z",        // plain RFC3339
	} {
		if got := formatCommentTime(in); !shape.MatchString(got) {
			t.Errorf("formatCommentTime(%q) = %q, want YYYY-MM-DD HH:MM:SS", in, got)
		}
	}
	// unparseable input degrades to the date prefix rather than erroring
	if got := formatCommentTime("garbage"); got != "garbage" {
		t.Errorf("expected raw passthrough for unparseable short input, got %q", got)
	}
	if got := formatCommentTime("2026-06-01 weird"); got != "2026-06-01" {
		t.Errorf("expected date-prefix fallback, got %q", got)
	}
}

// stubTaskHandler returns an HTTP handler that serves a single task on GET /tasks/{id}
// and an empty comment list on GET /comments.
func stubTaskHandler(task todoist.Task) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, task)
	})
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{})
	})
	return mux
}

func TestShow_DisplaysBasicDetails(t *testing.T) {
	task := todoist.Task{
		ID:      "task-abc",
		Content: "Write report",
		URL:     "https://todoist.com/app/task/task-abc",
	}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Write report", "p1", "")

	out, err := runCmd(t, "show", "task-abc")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "Write report") {
		t.Errorf("expected content in output, got: %q", out)
	}
	if !strings.Contains(out, "todoist.com") {
		t.Errorf("expected URL in output, got: %q", out)
	}
}

func TestShow_DisplaysDescription(t *testing.T) {
	task := todoist.Task{
		ID:          "task-desc",
		Content:     "Deploy service",
		Description: "Run the deployment script and verify health checks.",
	}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-desc", "Deploy service", "p1", "")

	out, err := runCmd(t, "show", "task-desc")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "Run the deployment script") {
		t.Errorf("expected description in output, got: %q", out)
	}
}

func TestShow_DisplaysDueDate(t *testing.T) {
	task := todoist.Task{
		ID:      "task-due",
		Content: "Submit report",
		Due:     &todoist.Due{Date: "2026-06-10", String: "Jun 10"},
	}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-due", "Submit report", "p1", "")

	out, err := runCmd(t, "show", "task-due")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "2026-06-10") {
		t.Errorf("expected due date in output, got: %q", out)
	}
	if !strings.Contains(out, "Jun 10") {
		t.Errorf("expected due string in output, got: %q", out)
	}
}

func TestShow_DisplaysDueDatetime(t *testing.T) {
	task := todoist.Task{
		ID:      "task-dt",
		Content: "Call client",
		Due:     &todoist.Due{Date: "2026-06-10", Datetime: "2026-06-10T14:00:00Z", String: "Jun 10 2pm"},
	}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-dt", "Call client", "p1", "")

	out, err := runCmd(t, "show", "task-dt")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	// When Datetime is set it takes precedence over Date in the output.
	if !strings.Contains(out, "2026-06-10T14:00:00Z") {
		t.Errorf("expected datetime in output, got: %q", out)
	}
}

func TestShow_DisplaysLabels(t *testing.T) {
	task := todoist.Task{
		ID:      "task-labels",
		Content: "Review PR",
		Labels:  []string{"urgent", "work"},
	}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-labels", "Review PR", "p1", "")

	out, err := runCmd(t, "show", "task-labels")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "urgent") {
		t.Errorf("expected label 'urgent' in output, got: %q", out)
	}
	if !strings.Contains(out, "work") {
		t.Errorf("expected label 'work' in output, got: %q", out)
	}
}

func TestShow_DisplaysPriorityAndProject(t *testing.T) {
	task := todoist.Task{ID: "t-pp", Content: "Write tests", ProjectID: "p-plat", Priority: 3}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p-plat", "Platform")
	hSeedTask(t, env.conn, "t-pp", "Write tests", "p-plat", "")

	out, err := runCmd(t, "show", "t-pp")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "project") || !strings.Contains(out, "Platform") {
		t.Errorf("expected project line with name, got: %q", out)
	}
	if !strings.Contains(out, "priority") || !strings.Contains(out, "3 (high)") {
		t.Errorf("expected 'priority  3 (high)', got: %q", out)
	}
}

func TestShow_HidesDefaultPriorityAndInbox(t *testing.T) {
	task := todoist.Task{ID: "t-def", Content: "Task", ProjectID: "p-inbox", Priority: 1}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p-inbox", "Inbox")
	hSeedTask(t, env.conn, "t-def", "Task", "p-inbox", "")

	out, err := runCmd(t, "show", "t-def")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if strings.Contains(out, "priority") {
		t.Errorf("default priority (1) must be hidden, got: %q", out)
	}
	if strings.Contains(out, "Inbox") {
		t.Errorf("Inbox project must be hidden, got: %q", out)
	}
}

func TestShow_DisplaysDeadline(t *testing.T) {
	task := todoist.Task{ID: "t-dl", Content: "Renew domain", Deadline: &todoist.Deadline{Date: "2026-09-30"}}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t-dl", "Renew domain", "p1", "")

	out, err := runCmd(t, "show", "t-dl")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "deadline") || !strings.Contains(out, "2026-09-30") {
		t.Errorf("expected deadline line, got: %q", out)
	}
}

func TestShow_NoDeadline_Omitted(t *testing.T) {
	task := todoist.Task{ID: "t-nodl", Content: "Task"}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t-nodl", "Task", "p1", "")

	out, err := runCmd(t, "show", "t-nodl")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if strings.Contains(out, "deadline") {
		t.Errorf("deadline must be omitted when unset, got: %q", out)
	}
}

// commentStub serves a task, its comments, and the project's collaborators.
func commentStub(task todoist.Task, comments []todoist.Comment, collabs []todoist.Collaborator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, task) })
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{Results: comments})
	})
	mux.HandleFunc("/projects/p1/collaborators", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Collaborator]{Results: collabs})
	})
	return mux
}

func TestShow_DisplaysComments(t *testing.T) {
	task := todoist.Task{ID: "task-c", Content: "Fix bug", ProjectID: "p1"}
	comments := []todoist.Comment{
		{ID: "c1", Content: "Needs a unit test too.", PostedAt: "2026-06-01T09:00:00Z", PostedUID: "u1"},
		{ID: "c2", Content: "Confirmed fixed.", PostedAt: "2026-06-02T10:30:00Z", PostedUID: "u2"},
	}
	collabs := []todoist.Collaborator{{ID: "u1", Name: "Alice"}, {ID: "u2", Name: "Bob"}}
	env := newTestEnv(t, commentStub(task, comments, collabs))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-c", "Fix bug", "p1", "")

	out, err := runCmd(t, "show", "task-c")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"Needs a unit test too.", "Confirmed fixed.", "Alice", "Bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
	// A full date-time (with time component) must be rendered — TZ-independent check.
	if !regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`).MatchString(out) {
		t.Errorf("expected a full date-time on each comment, got: %q", out)
	}
}

func TestShow_CommentAuthorFallsBackToUID(t *testing.T) {
	task := todoist.Task{ID: "task-c", Content: "Fix bug", ProjectID: "p1"}
	comments := []todoist.Comment{
		{ID: "c1", Content: "From a former collaborator.", PostedAt: "2026-06-01T09:00:00Z", PostedUID: "ghost-uid"},
	}
	// collaborators list does not contain ghost-uid
	env := newTestEnv(t, commentStub(task, comments, []todoist.Collaborator{{ID: "u1", Name: "Alice"}}))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-c", "Fix bug", "p1", "")

	out, err := runCmd(t, "show", "task-c")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "ghost-uid") {
		t.Errorf("expected fallback to the raw uid when unresolved, got: %q", out)
	}
}

func TestShow_DisplaysSubtasks(t *testing.T) {
	task := todoist.Task{ID: "task-parent", Content: "Epic task"}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-parent", "Epic task", "p1", "")
	hSeedSubtask(t, env.conn, "sub-1", "Subtask one", "p1", "task-parent")
	hSeedSubtask(t, env.conn, "sub-2", "Subtask two", "p1", "task-parent")

	out, err := runCmd(t, "show", "task-parent")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, "Subtask one") {
		t.Errorf("expected subtask one in output, got: %q", out)
	}
	if !strings.Contains(out, "Subtask two") {
		t.Errorf("expected subtask two in output, got: %q", out)
	}
}

func TestShow_ResolvesPrefix(t *testing.T) {
	var receivedID string
	task := todoist.Task{ID: "task-xyz-full", Content: "Refactor auth"}
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		receivedID = strings.TrimPrefix(r.URL.Path, "/tasks/")
		writeJSON(w, task)
	})
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[todoist.Comment]{})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-xyz-full", "Refactor auth", "p1", "")

	// Resolve by task name.
	if _, err := runCmd(t, "show", "Refactor auth"); err != nil {
		t.Fatalf("show: %v", err)
	}
	if receivedID != "task-xyz-full" {
		t.Errorf("expected API call with full ID 'task-xyz-full', got %q", receivedID)
	}
}

func TestShow_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	newTestEnv(t, mux)

	_, err := runCmd(t, "show", "nonexistent-id")
	if err == nil {
		t.Fatal("expected error when API returns 404, got nil")
	}
}

func TestShow_NoDescription_Omitted(t *testing.T) {
	task := todoist.Task{ID: "task-nodesc", Content: "Simple task", Description: ""}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-nodesc", "Simple task", "p1", "")

	out, err := runCmd(t, "show", "task-nodesc")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	// Output should not have a blank description section.
	if strings.Contains(out, "\n\n\n") {
		t.Errorf("unexpected extra blank lines in output: %q", out)
	}
}

func TestShow_NoDueDate_Omitted(t *testing.T) {
	task := todoist.Task{ID: "task-nodue", Content: "Timeless task", Due: nil}
	env := newTestEnv(t, stubTaskHandler(task))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-nodue", "Timeless task", "p1", "")

	out, err := runCmd(t, "show", "task-nodue")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if strings.Contains(out, "due") {
		t.Errorf("expected no due date in output, got: %q", out)
	}
}
