//go:build live

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// Live tests run the CLI against the real Todoist API with two dedicated test
// accounts. Never point them at a real account: they create and delete data.
//
//	TODOIST_TOKEN_A  main test account
//	TODOIST_TOKEN_B  second account; must share liveSharedProject with A
//
// Run with: make test-live

const (
	livePrefix        = "tdc-live-"
	liveSharedProject = "tdc-live-shared"
)

var sweepOnce sync.Once

// liveEnv points the CLI at the real API as the account behind tokenVar, with
// a fresh cache. Calling it again in the same test switches accounts.
func liveEnv(t *testing.T, tokenVar string) string {
	t.Helper()
	token := os.Getenv(tokenVar)
	if token == "" {
		t.Skipf("%s not set", tokenVar)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("TODOIST_TOKEN", token)
	t.Setenv("TODOIST_API_BASE", "")
	sweepOnce.Do(func() { sweepStale(t, token) })
	return token
}

// sweepStale deletes projects left behind by an earlier run that died before
// its cleanup ran.
func sweepStale(t *testing.T, token string) {
	client := todoist.New(token)
	projects, err := client.GetProjects(context.Background())
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	for _, p := range projects {
		if strings.HasPrefix(p.Name, livePrefix) && p.Name != liveSharedProject {
			client.DeleteProject(context.Background(), p.ID)
		}
	}
}

func liveName(kind string) string {
	return fmt.Sprintf("%s%s-%d", livePrefix, kind, time.Now().UnixNano())
}

// liveProject creates a throwaway project that is deleted, with everything in
// it, when the test ends.
func liveProject(t *testing.T, token string) (id, name string) {
	t.Helper()
	client := todoist.New(token)
	name = liveName("proj")
	p, err := client.CreateProject(context.Background(), todoist.CreateProjectRequest{Name: name})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() { client.DeleteProject(context.Background(), p.ID) })
	return p.ID, name
}

// liveCreateSection covers the one write the CLI has no command for.
func liveCreateSection(t *testing.T, token, projectID, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name, "project_id": projectID})
	req, _ := http.NewRequest("POST", "https://api.todoist.com/api/v1/sections", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("create section: HTTP %d", resp.StatusCode)
	}
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runCmd(t, args...)
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

type liveTask struct {
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Section     *string  `json:"section"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Due         *struct {
		Date string `json:"date"`
	} `json:"due"`
	Deadline *string `json:"deadline"`
	Subtasks []struct {
		ID string `json:"id"`
	} `json:"subtasks"`
	Comments []struct {
		Author  string `json:"author"`
		Content string `json:"content"`
	} `json:"comments"`
}

func showTask(t *testing.T, id string) liveTask {
	t.Helper()
	var task liveTask
	out := mustRun(t, "show", id, "--json")
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("show --json: %v\n%s", err, out)
	}
	return task
}

func TestLiveTaskLifecycle(t *testing.T) {
	token := liveEnv(t, "TODOIST_TOKEN_A")
	projectID, project := liveProject(t, token)
	liveCreateSection(t, token, projectID, "Todo")
	liveCreateSection(t, token, projectID, "Doing")
	label := liveName("label")
	renamed := liveName("renamed")
	deadline := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	mustRun(t, "sync")
	mustRun(t, "cd", project)

	parent := strings.TrimSpace(mustRun(t, "add", "alpha", "-l", label, "-D", "tomorrow",
		"-P", "4", "-d", "why", "--deadline", deadline, "-s", "Todo"))
	child := strings.TrimSpace(mustRun(t, "add", "alpha child", "--parent", parent))
	other := strings.TrimSpace(mustRun(t, "add", "beta"))
	mustRun(t, "sync")

	if out := mustRun(t, "ls", "--ids"); !strings.Contains(out, parent) || !strings.Contains(out, other) {
		t.Errorf("ls --ids missing tasks:\n%s", out)
	}
	if out := mustRun(t, "search", "-i", "alpha child"); !strings.Contains(out, child) {
		t.Errorf("search missing subtask:\n%s", out)
	}

	listed := decodeJSON[[]jsonTask](t, mustRun(t, "ls", "--json"))
	if i := slices.IndexFunc(listed, func(j jsonTask) bool { return j.ID == parent }); i < 0 ||
		listed[i].Deadline == nil || *listed[i].Deadline != deadline || !slices.Contains(listed[i].Labels, label) {
		t.Errorf("ls --json after sync: %+v", listed)
	}
	if sections := decodeJSON[[]map[string]string](t, mustRun(t, "sections", "--json")); len(sections) != 2 {
		t.Errorf("sections --json: %v", sections)
	}

	got := showTask(t, parent)
	if got.Priority != 4 || got.Description != "why" || got.Due == nil ||
		got.Deadline == nil || *got.Deadline != deadline ||
		got.Section == nil || *got.Section != "Todo" ||
		!slices.Contains(got.Labels, label) || len(got.Subtasks) != 1 {
		t.Errorf("show after add: %+v", got)
	}

	mustRun(t, "edit", parent, "-c", "alpha edited", "-P", "2", "--due", "")
	mustRun(t, "mv", parent, "Doing")
	got = showTask(t, parent)
	if got.Content != "alpha edited" || got.Priority != 2 || got.Due != nil ||
		got.Section == nil || *got.Section != "Doing" {
		t.Errorf("show after edit/mv: %+v", got)
	}

	mustRun(t, "comment", parent, "hello from the live test")
	comments := mustRun(t, "comment", "ls", parent)
	commentID, _, _ := strings.Cut(comments, "\t")
	if got := showTask(t, parent); len(got.Comments) != 1 || got.Comments[0].Author == "" {
		t.Errorf("show comments: %+v", got.Comments)
	}
	if c := decodeJSON[[]jsonComment](t, mustRun(t, "comment", "ls", "--json", parent)); len(c) != 1 || c[0].Author == "" {
		t.Errorf("comment ls --json: %+v", c)
	}
	mustRun(t, "comment", "rm", commentID)
	if out := mustRun(t, "comment", "ls", parent); out != "" {
		t.Errorf("comment still listed after rm:\n%s", out)
	}

	if out := mustRun(t, "labels"); !strings.Contains(out, label+"\tshared") {
		t.Errorf("labels missing %s:\n%s", label, out)
	}
	mustRun(t, "labels", "rename", label, renamed)
	if got := showTask(t, parent); !slices.Contains(got.Labels, renamed) {
		t.Errorf("labels after rename: %v", got.Labels)
	}

	mustRun(t, "done", child)
	if out := mustRun(t, "ls", "--done", "today"); !strings.Contains(out, "alpha child") {
		t.Errorf("ls --done today missing completed subtask:\n%s", out)
	}

	mustRun(t, "rm", other)
	mustRun(t, "sync")
	if out := mustRun(t, "ls", "--ids"); strings.Contains(out, other) {
		t.Errorf("deleted task still listed:\n%s", out)
	}

	sub := liveName("sub")
	mustRun(t, "projects", "add", sub, "--parent", project)
	mustRun(t, "projects", "mv", sub, sub+"-moved")
	mustRun(t, "projects", "rm", sub+"-moved")
	stats := decodeJSON[map[string]any](t, mustRun(t, "stats", "--json"))
	if stats["done_today"] == nil || stats["done_today"].(float64) < 1 {
		t.Errorf("stats --json done_today: %v", stats)
	}
}

// TestLiveSharedProject covers what needs a second person: comment authors
// resolved through /collaborators and labels created by someone else.
func TestLiveSharedProject(t *testing.T) {
	if os.Getenv("TODOIST_TOKEN_A") == "" {
		t.Skip("TODOIST_TOKEN_A not set")
	}
	tokenB := liveEnv(t, "TODOIST_TOKEN_B")
	mustRun(t, "sync")
	if _, err := runCmd(t, "cd", liveSharedProject); err != nil {
		t.Fatalf("account B cannot see %q — share it from A and accept the invite: %v", liveSharedProject, err)
	}

	label := liveName("label-b")
	taskID := strings.TrimSpace(mustRun(t, "add", "from B", "-l", label))
	t.Cleanup(func() { todoist.New(tokenB).DeleteTask(context.Background(), taskID) })
	mustRun(t, "comment", taskID, "comment by B")
	comments, err := todoist.New(tokenB).GetComments(context.Background(), taskID)
	if err != nil || len(comments) != 1 {
		t.Fatalf("B's comments: %v %v", comments, err)
	}

	liveEnv(t, "TODOIST_TOKEN_A")
	mustRun(t, "sync")
	got := showTask(t, taskID)
	if !slices.Contains(got.Labels, label) {
		t.Errorf("A does not see B's label: %v", got.Labels)
	}
	if len(got.Comments) != 1 || got.Comments[0].Author == comments[0].PostedUID {
		t.Errorf("B's comment author not resolved to a name: %+v", got.Comments)
	}
	if out := mustRun(t, "labels"); !strings.Contains(out, label+"\tshared") {
		t.Errorf("A's labels missing %s as shared:\n%s", label, out)
	}
}
