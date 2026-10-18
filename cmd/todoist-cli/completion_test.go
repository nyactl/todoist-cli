package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// complete runs cobra's hidden __complete command the way a shell does on
// <TAB> and returns the suggestions and the directive line.
func complete(t *testing.T, args ...string) (suggestions []string, directive string) {
	t.Helper()
	out, err := runCmd(t, append([]string{"__complete"}, args...)...)
	if err != nil {
		t.Fatalf("__complete %v: %v", args, err)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(line, ":") {
			directive = line
			continue
		}
		suggestions = append(suggestions, line)
	}
	return suggestions, directive
}

func seedCompletionData(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Home")
	env.conn.Exec(`INSERT INTO projects (id, name, is_archived) VALUES ('p3', 'Old stuff', 1)`)
	hSeedSection(t, env.conn, "s1", "Todo", "p1", 1)
	hSeedSection(t, env.conn, "s2", "Doing", "p1", 2)
	hSeedSection(t, env.conn, "s3", "Garden", "p2", 1)
	env.conn.Exec(`INSERT INTO labels (id, name) VALUES ('l1', 'urgent')`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Pay invoice', 'p1', date('now', 'localtime'))`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id) VALUES ('t2', 'Refactor parser', 'p1')`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2', 'someday')`)
}

func TestComplete_ProjectsSkipArchived(t *testing.T) {
	env := newTestEnv(t, nil)
	seedCompletionData(t, env)
	got, dir := complete(t, "add", "x", "--project", "")
	want := []string{"Work\tp1", "Home\tp2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("projects = %q, want %q", got, want)
	}
	if dir != ":4" {
		t.Errorf("directive = %q, want :4 (no file completion)", dir)
	}
}

func TestComplete_LabelsIncludePersonalAndShared(t *testing.T) {
	env := newTestEnv(t, nil)
	seedCompletionData(t, env)
	got, _ := complete(t, "ls", "--label", "")
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "urgent\tpersonal") || !strings.Contains(joined, "someday\tshared") {
		t.Errorf("labels = %q", got)
	}
}

func TestComplete_SectionsFollowProjectFlagThenContext(t *testing.T) {
	env := newTestEnv(t, nil)
	seedCompletionData(t, env)

	if got, _ := complete(t, "add", "x", "--section", ""); len(got) != 0 {
		t.Errorf("no project: sections = %q, want none", got)
	}
	if got, _ := complete(t, "add", "x", "--project", "Home", "--section", ""); strings.Join(got, "|") != "Garden" {
		t.Errorf("--project Home: sections = %q", got)
	}
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := complete(t, "add", "x", "--section", ""); strings.Join(got, "|") != "Todo|Doing" {
		t.Errorf("context Work: sections = %q", got)
	}
	if got, _ := complete(t, "mv", "t1", "--project", "Home", "--section", ""); strings.Join(got, "|") != "Garden" {
		t.Errorf("mv --project Home --section: %q", got)
	}
}

func TestComplete_TaskArgument(t *testing.T) {
	env := newTestEnv(t, nil)
	seedCompletionData(t, env)

	got, _ := complete(t, "done", "")
	if strings.Join(got, "|") != "Pay invoice\tt1" {
		t.Errorf("no context: tasks = %q, want only the due task", got)
	}
	if got, _ := complete(t, "done", "t1", ""); len(got) != 0 {
		t.Errorf("second argument: %q, want none", got)
	}
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatal(err)
	}
	got, _ = complete(t, "show", "")
	if len(got) != 2 {
		t.Errorf("context Work: tasks = %q, want both project tasks", got)
	}
}

func TestComplete_MvSecondArgumentIsSection(t *testing.T) {
	env := newTestEnv(t, nil)
	seedCompletionData(t, env)
	if got, _ := complete(t, "mv", "t1", ""); len(got) != 0 {
		t.Errorf("no context: %q, want none", got)
	}
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := complete(t, "mv", "t1", ""); strings.Join(got, "|") != "Todo|Doing" {
		t.Errorf("mv sections = %q", got)
	}
	if got, _ := complete(t, "mv", ""); len(got) != 2 {
		t.Errorf("mv first argument should complete tasks, got %q", got)
	}
}

func TestComplete_DonePeriods(t *testing.T) {
	newTestEnv(t, nil)
	got, _ := complete(t, "ls", "--done", "")
	if len(got) == 0 || got[0] != "today" {
		t.Errorf("periods = %q", got)
	}
}

func TestComplete_OpenOffersTasksCompletedToday(t *testing.T) {
	newTestEnv(t, completedHandler(`{"task_id":"tc1","content":"finished thing","project_id":"p1","completed_at":"2026-08-12T09:00:00Z"}`))
	got, _ := complete(t, "open", "")
	if strings.Join(got, "|") != "tc1\tfinished thing" {
		t.Errorf("open completions = %q", got)
	}
	if got, _ := complete(t, "open", "tc1", ""); len(got) != 0 {
		t.Errorf("second argument: %q", got)
	}
}

func TestComplete_OpenReportsAPIFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/completed", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	newTestEnv(t, mux)
	if _, dir := complete(t, "open", ""); dir != ":1" {
		t.Errorf("directive = %q, want :1 (error)", dir)
	}
}
