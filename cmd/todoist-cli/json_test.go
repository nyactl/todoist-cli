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

func decodeJSON[T any](t *testing.T, out string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	return v
}

func mustJSON(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runCmd(t, args...)
	if err != nil {
		t.Fatalf("%s: %v", strings.Join(args, " "), err)
	}
	return out
}

func setContext(t *testing.T, id, name string) {
	t.Helper()
	if err := state.Save(&state.State{ProjectID: id, ProjectName: name}); err != nil {
		t.Fatalf("set context: %v", err)
	}
}

// seedRichTask stores a task carrying every field the JSON task object exposes.
func seedRichTask(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Doing", "p1", 1)
	if _, err := env.conn.ExecContext(context.Background(), `
		INSERT INTO tasks (id, content, description, project_id, section_id, priority,
		                   due_date, due_string, due_is_recurring, deadline_date, url)
		VALUES ('6Jf8VQXxpwv56VQ7', 'Write report', 'for Q4', 'p1', 's1', 4,
		        date('now', 'localtime'), 'every day', 1, '2026-12-31', 'https://example.test/t')`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('6Jf8VQXxpwv56VQ7', 'focus')`)
	hSeedSubtask(t, env.conn, "7Kg9WRYyqxw67WR8", "Draft outline", "p1", "6Jf8VQXxpwv56VQ7")
}

func assertRichTask(t *testing.T, got jsonTask) {
	t.Helper()
	if got.ID != "6Jf8VQXxpwv56VQ7" || got.Content != "Write report" || got.Description != "for Q4" ||
		got.Project == nil || *got.Project != "Work" || got.Section == nil || *got.Section != "Doing" ||
		got.ParentID != nil || got.Priority != 4 || len(got.Labels) != 1 || got.Labels[0] != "focus" ||
		got.Due == nil || !got.Due.Recurring || got.Due.String == nil || *got.Due.String != "every day" ||
		got.Deadline == nil || *got.Deadline != "2026-12-31" || got.URL != "https://example.test/t" {
		b, _ := json.Marshal(got)
		t.Errorf("unexpected task object: %s", b)
	}
}

func TestJSON_LsScoped_FullTaskObject(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)
	setContext(t, "p1", "Work")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "ls", "--json"))
	if len(got) != 1 {
		t.Fatalf("want 1 top-level task, got %d", len(got))
	}
	assertRichTask(t, got[0])
}

func TestJSON_LsBoardAndIDs_SameFlatArray(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)
	setContext(t, "p1", "Work")

	plain := mustJSON(t, "ls", "--json")
	if board := mustJSON(t, "ls", "--json", "--board"); board != plain {
		t.Errorf("--board changed JSON output:\n%s\nvs\n%s", board, plain)
	}
	if ids := mustJSON(t, "ls", "--json", "--ids"); ids != plain {
		t.Errorf("--ids changed JSON output:\n%s\nvs\n%s", ids, plain)
	}
}

func TestJSON_LsAgenda_NoContext(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "ls", "--json"))
	if len(got) != 1 {
		t.Fatalf("want today's task, got %d", len(got))
	}
	assertRichTask(t, got[0])
}

func TestJSON_LsLabelFilter(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)
	hSeedTask(t, env.conn, "other", "Unlabelled", "p1", "")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "ls", "--json", "-l", "focus"))
	if len(got) != 1 || got[0].ID != "6Jf8VQXxpwv56VQ7" {
		t.Errorf("want only the labelled task, got %+v", got)
	}
}

func TestJSON_LsEmpty_PrintsEmptyArrayNotHint(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	setContext(t, "p1", "Work")

	for _, args := range [][]string{{"ls", "--json"}, {"ls", "--json", "-g"}, {"search", "--json", "nothing"}} {
		if out := strings.TrimSpace(mustJSON(t, args...)); out != "[]" {
			t.Errorf("%v: want [], got %q", args, out)
		}
	}
}

func TestJSON_Search_IncludesSubtaskWithParent(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "search", "--json", "outline"))
	if len(got) != 1 || got[0].ID != "7Kg9WRYyqxw67WR8" ||
		got[0].ParentID == nil || *got[0].ParentID != "6Jf8VQXxpwv56VQ7" || got[0].Labels == nil {
		t.Errorf("unexpected search result: %+v", got)
	}
}

func TestJSON_LsDone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"task_id":"9Lh0XSZzryx78XS9","content":"Ship it","project_id":"p1","section_id":"s1","completed_at":"2026-10-18T08:00:00.000000Z"}],
			"projects":{"p1":{"name":"Work"}},"sections":{"s1":{"name":"Doing"}}}`))
	})
	newTestEnv(t, mux)

	type completed struct {
		ID          string  `json:"id"`
		Content     string  `json:"content"`
		Project     *string `json:"project"`
		Section     *string `json:"section"`
		CompletedAt string  `json:"completed_at"`
	}
	got := decodeJSON[[]completed](t, mustJSON(t, "ls", "--done", "today", "--json"))
	if len(got) != 1 || got[0].ID != "9Lh0XSZzryx78XS9" || got[0].Content != "Ship it" ||
		got[0].Project == nil || *got[0].Project != "Work" || got[0].Section == nil || *got[0].Section != "Doing" ||
		got[0].CompletedAt != "2026-10-18T08:00:00.000000Z" {
		t.Errorf("unexpected completed entry: %+v", got)
	}
}

func TestJSON_LsDoneEmpty(t *testing.T) {
	env := newTestEnv(t, completedHandler(""))
	hSeedProject(t, env.conn, "p1", "Work")
	setContext(t, "p1", "Work")

	if out := strings.TrimSpace(mustJSON(t, "ls", "--done", "today", "--json")); out != "[]" {
		t.Errorf("want [], got %q", out)
	}
}

func TestJSON_Projects(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProjectOrd(t, env.conn, "p1", "Dev", 1)
	hSeedSubproject(t, env.conn, "p2", "CLI", "p1")
	env.conn.ExecContext(context.Background(), `UPDATE projects SET is_favorite = 1, ord = 2 WHERE id = 'p2'`)
	hSeedArchivedProject(t, env.conn, "p3", "Old")

	type project struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		ParentID   *string `json:"parent_id"`
		IsFavorite bool    `json:"is_favorite"`
	}
	got := decodeJSON[[]project](t, mustJSON(t, "projects", "--json"))
	if len(got) != 2 || got[0].ParentID != nil || got[0].IsFavorite ||
		got[1].ParentID == nil || *got[1].ParentID != "p1" || !got[1].IsFavorite {
		t.Errorf("unexpected projects: %+v", got)
	}
}

func TestJSON_Sections(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Todo", "p1", 1)
	hSeedSection(t, env.conn, "s2", "Doing", "p1", 2)
	setContext(t, "p1", "Work")

	type section struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
	}
	got := decodeJSON[[]section](t, mustJSON(t, "sections", "--json"))
	if len(got) != 2 || got[0] != (section{"s1", "Todo", "p1"}) || got[1] != (section{"s2", "Doing", "p1"}) {
		t.Errorf("unexpected sections: %+v", got)
	}
}

func TestJSON_Labels_SharedHaveNullID(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedLabel(t, env.conn, "l1", "work", 1)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Task", "p1", "")
	env.conn.ExecContext(context.Background(), `INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'team')`)

	type label struct {
		ID   *string `json:"id"`
		Name string  `json:"name"`
		Kind string  `json:"kind"`
	}
	got := decodeJSON[[]label](t, mustJSON(t, "labels", "--json"))
	if len(got) != 2 || got[0].ID == nil || *got[0].ID != "l1" || got[0].Kind != "personal" ||
		got[1].ID != nil || got[1].Name != "team" || got[1].Kind != "shared" {
		t.Errorf("unexpected labels: %+v", got)
	}
}

func TestJSON_Comments_FullContentAndAuthor(t *testing.T) {
	task := todoist.Task{ID: "task-c", Content: "Fix bug", ProjectID: "p1"}
	comments := []todoist.Comment{
		{ID: "c1", TaskID: "task-c", Content: "line one\nline two", PostedAt: "2026-06-01T09:00:00Z", PostedUID: "u1"},
	}
	mux := commentStub(task, comments, []todoist.Collaborator{{ID: "u1", Name: "Alice"}})
	mux.(*http.ServeMux).HandleFunc("/comments/c1", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, comments[0]) })
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-c", "Fix bug", "p1", "")

	want := jsonComment{ID: "c1", TaskID: "task-c", PostedAt: "2026-06-01T09:00:00Z", Author: "Alice", Content: "line one\nline two"}
	for _, args := range [][]string{{"comment", "ls", "--json", "task-c"}, {"comment", "show", "--json", "c1"}} {
		got := decodeJSON[[]jsonComment](t, mustJSON(t, args...))
		if len(got) != 1 || got[0] != want {
			t.Errorf("%v: got %+v", args, got)
		}
	}
}

func TestJSON_Stats(t *testing.T) {
	env := newTestEnv(t, completedHandler(`{"task_id":"x","content":"Done","project_id":"p1","completed_at":"2000-01-01T00:00:00Z"}`))
	hSeedProject(t, env.conn, "p1", "Work")
	env.conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Late', 'p1', date('now', 'localtime', '-2 day'))`)
	setContext(t, "p1", "Work")

	type stats struct {
		Project   *string `json:"project"`
		Overdue   int     `json:"overdue"`
		OpenTotal int     `json:"open_total"`
		DoneToday *int    `json:"done_today"`
		DoneWeek  *int    `json:"done_week"`
	}
	got := decodeJSON[stats](t, mustJSON(t, "stats", "--json"))
	if got.Project == nil || *got.Project != "Work" || got.Overdue != 1 || got.OpenTotal != 1 ||
		got.DoneToday == nil || *got.DoneToday != 0 || got.DoneWeek == nil || *got.DoneWeek != 1 {
		t.Errorf("unexpected stats: %+v", got)
	}
}

func TestJSON_Stats_DoneCountsNullWhenAPIFails(t *testing.T) {
	newTestEnv(t, http.NotFoundHandler())

	out := mustJSON(t, "stats", "--json")
	got := decodeJSON[map[string]any](t, out)
	if got["done_today"] != nil || got["done_week"] != nil {
		t.Errorf("want null done counts, got %s", out)
	}
	if _, ok := got["done_today"]; !ok {
		t.Errorf("done_today must be present as null, got %s", out)
	}
}

func TestJSON_Context(t *testing.T) {
	env := newTestEnv(t, nil)
	if out := strings.TrimSpace(mustJSON(t, "context", "--json")); out != "null" {
		t.Errorf("no context: want null, got %q", out)
	}
	hSeedProject(t, env.conn, "p1", "Work")
	setContext(t, "p1", "Work")
	got := decodeJSON[map[string]string](t, mustJSON(t, "context", "--json"))
	if got["id"] != "p1" || got["name"] != "Work" {
		t.Errorf("unexpected context: %v", got)
	}
}

func TestSync_StoresDeadline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{{ID: "p1", Name: "Work"}}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{
		{ID: "t1", Content: "Tax return", ProjectID: "p1", Priority: 1, Deadline: &todoist.Deadline{Date: "2027-05-31"}},
	}))
	newTestEnv(t, mux)
	mustJSON(t, "sync")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "search", "--json", "Tax"))
	if len(got) != 1 || got[0].Deadline == nil || *got[0].Deadline != "2027-05-31" {
		t.Errorf("deadline not cached by sync: %+v", got)
	}
}

func TestAdd_StoresDeadline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, todoist.Task{ID: "t9", Content: "Renew passport", Priority: 1, Deadline: &todoist.Deadline{Date: "2027-01-15"}})
	})
	newTestEnv(t, mux)
	mustJSON(t, "add", "Renew passport", "--deadline", "2027-01-15")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "search", "--json", "passport"))
	if len(got) != 1 || got[0].Deadline == nil || *got[0].Deadline != "2027-01-15" {
		t.Errorf("deadline not cached by add: %+v", got)
	}
}

func TestJSON_LsRecursive_IncludesSubprojects(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Dev")
	hSeedSubproject(t, env.conn, "p2", "CLI", "p1")
	hSeedTask(t, env.conn, "t1", "Parent task", "p1", "")
	hSeedTask(t, env.conn, "t2", "Child project task", "p2", "")
	setContext(t, "p1", "Dev")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "ls", "--json", "-r"))
	projects := map[string]bool{}
	for _, task := range got {
		projects[*task.Project] = true
	}
	if len(got) != 2 || !projects["Dev"] || !projects["CLI"] {
		t.Errorf("want tasks from both projects, got %+v", got)
	}
}

func TestJSON_LsPriorityFilter(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)
	hSeedTask(t, env.conn, "low", "Low priority", "p1", "")
	setContext(t, "p1", "Work")

	got := decodeJSON[[]jsonTask](t, mustJSON(t, "ls", "--json", "-P", "4"))
	if len(got) != 1 || got[0].ID != "6Jf8VQXxpwv56VQ7" {
		t.Errorf("want only the priority-4 task, got %+v", got)
	}
}

func TestJSON_LsLabelEmptyInContext_PrintsEmptyArray(t *testing.T) {
	env := newTestEnv(t, nil)
	seedRichTask(t, env)
	hSeedProject(t, env.conn, "p9", "Home")
	setContext(t, "p9", "Home")

	if out := strings.TrimSpace(mustJSON(t, "ls", "--json", "-l", "focus")); out != "[]" {
		t.Errorf("want [], got %q", out)
	}
}

func TestEdit_ClearingDeadlineClearsCache(t *testing.T) {
	stub := makeEditStub(t, "6Jf8VQXxpwv56VQ7", func(map[string]any) {},
		todoist.Task{ID: "6Jf8VQXxpwv56VQ7", Content: "Write report", ProjectID: "p1", Priority: 4})
	env := newTestEnv(t, stub)
	seedRichTask(t, env)

	mustJSON(t, "edit", "6Jf8VQXxpwv56VQ7", "--deadline", "")
	got := decodeJSON[[]jsonTask](t, mustJSON(t, "search", "--json", "Write report"))
	if len(got) != 1 || got[0].Deadline != nil {
		t.Errorf("deadline still cached after clearing: %+v", got)
	}
}
