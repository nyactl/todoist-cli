package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestComment_PostsToAPI(t *testing.T) {
	var gotBody map[string]string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, todoist.Comment{
			ID:       "c1",
			TaskID:   "task-abc",
			Content:  gotBody["content"],
			PostedAt: "2026-06-04T12:00:00Z",
		})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Fix bug", "p1", "")

	_, err := runCmd(t, "comment", "Fix bug", "Needs a unit test too.")
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if gotBody["task_id"] != "task-abc" {
		t.Errorf("expected task_id 'task-abc' sent to API, got %q", gotBody["task_id"])
	}
	if gotBody["content"] != "Needs a unit test too." {
		t.Errorf("expected comment content sent to API, got %q", gotBody["content"])
	}
}

func TestComment_MultiWordText(t *testing.T) {
	var gotContent string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		gotContent = body["content"]
		writeJSON(w, todoist.Comment{ID: "c1", TaskID: "t1", Content: gotContent})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Deploy service", "p1", "")

	if _, err := runCmd(t, "comment", "Deploy service", "check", "the", "logs", "first"); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if gotContent != "check the logs first" {
		t.Errorf("expected joined multi-word content, got %q", gotContent)
	}
}

func TestComment_PrintsConfirmation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, todoist.Comment{ID: "c1", TaskID: "task-xyz", Content: "looks good"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-xyz", "Review PR", "p1", "")

	out, err := runCmd(t, "comment", "Review PR", "looks good")
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if !strings.Contains(out, "commented") {
		t.Errorf("expected 'commented' in output, got: %q", out)
	}
	if !strings.Contains(out, "looks good") {
		t.Errorf("expected comment text in output, got: %q", out)
	}
}

func TestComment_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "comment", "nonexistent-task", "some comment")
	if err == nil {
		t.Fatal("expected error for unknown task, got nil")
	}
}

func TestComment_EmptyText_Errors(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Some task", "p1", "")

	_, err := runCmd(t, "comment", "Some task", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only comment, got nil")
	}
}

func TestComment_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Some task", "p1", "")

	_, err := runCmd(t, "comment", "Some task", "hello")
	if err == nil {
		t.Fatal("expected error when API returns 403, got nil")
	}
}

// --- comment ls ---

func TestCommentLs_ListsIDTimestampAndFirstLine(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("task_id"); got != "task-abc" {
			t.Errorf("expected task_id 'task-abc' in query, got %q", got)
		}
		writeJSON(w, map[string]any{
			"results": []todoist.Comment{
				{ID: "cmt-1", TaskID: "task-abc", Content: "first line\nsecond line", PostedAt: "2026-06-04T12:00:00.000000Z"},
				{ID: "cmt-2", TaskID: "task-abc", Content: "single line", PostedAt: "2026-06-05T09:30:00.000000Z"},
			},
			"next_cursor": nil,
		})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Fix bug", "p1", "")

	out, err := runCmd(t, "comment", "ls", "Fix bug")
	if err != nil {
		t.Fatalf("comment ls: %v", err)
	}
	if !strings.Contains(out, "cmt-1\t") || !strings.Contains(out, "cmt-2\t") {
		t.Errorf("expected full comment IDs in output, got: %q", out)
	}
	if !strings.Contains(out, "first line") {
		t.Errorf("expected first line of multi-line comment, got: %q", out)
	}
	if strings.Contains(out, "second line") {
		t.Errorf("expected only the first line, but second line leaked: %q", out)
	}
}

func TestCommentLs_EmptyTaskPrintsNothing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"results": []todoist.Comment{}, "next_cursor": nil})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "No comments here", "p1", "")

	out, err := runCmd(t, "comment", "ls", "No comments here")
	if err != nil {
		t.Fatalf("comment ls: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for a task with no comments, got: %q", out)
	}
}

func TestCommentLs_UnknownTask_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "comment", "ls", "no-such-task")
	if err == nil {
		t.Fatal("expected error for unknown task, got nil")
	}
}

// --- comment rm ---

// commentByIDStub serves GET and DELETE on /comments/{id}, recording deletes.
func commentByIDStub(t *testing.T, deleted *[]string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/comments/")
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, todoist.Comment{ID: id, Content: id + " body\nsecond line", PostedAt: "2026-06-04T12:00:00.000000Z"})
		case http.MethodDelete:
			*deleted = append(*deleted, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func TestCommentRm_DeletesAndEchoesDetails(t *testing.T) {
	var deleted []string
	newTestEnv(t, commentByIDStub(t, &deleted))

	out, err := runCmd(t, "comment", "rm", "cmt-xyz")
	if err != nil {
		t.Fatalf("comment rm: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "cmt-xyz" {
		t.Errorf("expected DELETE of 'cmt-xyz', got %v", deleted)
	}
	// Echo must carry the ID, timestamp and first line — first line only (#26).
	if !strings.Contains(out, "deleted: cmt-xyz\t") {
		t.Errorf("expected 'deleted: cmt-xyz' with details, got: %q", out)
	}
	if !strings.Contains(out, "cmt-xyz body") {
		t.Errorf("expected first line in echo, got: %q", out)
	}
	if strings.Contains(out, "second line") {
		t.Errorf("expected only first line in echo, got: %q", out)
	}
}

func TestCommentRm_MultipleIDs(t *testing.T) {
	var deleted []string
	newTestEnv(t, commentByIDStub(t, &deleted))

	if _, err := runCmd(t, "comment", "rm", "c1", "c2", "c3"); err != nil {
		t.Fatalf("comment rm: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("expected 3 deletes, got %v", deleted)
	}
	for i, want := range []string{"c1", "c2", "c3"} {
		if deleted[i] != want {
			t.Errorf("delete %d: expected %q, got %q", i, want, deleted[i])
		}
	}
}

func TestCommentRm_All_PromptConfirmed(t *testing.T) {
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []todoist.Comment{
				{ID: "a1", Content: "first", PostedAt: "2026-06-04T12:00:00.000000Z"},
				{ID: "a2", Content: "second", PostedAt: "2026-06-05T12:00:00.000000Z"},
			},
			"next_cursor": nil,
		})
	})
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/comments/"))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Clean me", "p1", "")

	setStdin(t, "y\n")
	out, err := runCmd(t, "comment", "rm", "--all", "Clean me")
	if err != nil {
		t.Fatalf("comment rm --all: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected both comments deleted, got %v", deleted)
	}
	if !strings.Contains(out, "delete all 2 comment(s)") {
		t.Errorf("expected confirmation prompt, got: %q", out)
	}
}

func TestCommentRm_All_Aborted(t *testing.T) {
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results":     []todoist.Comment{{ID: "a1", Content: "keep me", PostedAt: "2026-06-04T12:00:00.000000Z"}},
			"next_cursor": nil,
		})
	})
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/comments/"))
			w.WriteHeader(http.StatusNoContent)
		}
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Keep", "p1", "")

	setStdin(t, "n\n")
	out, err := runCmd(t, "comment", "rm", "--all", "Keep")
	if err != nil {
		t.Fatalf("comment rm --all (abort): %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected no deletes when aborted, got %v", deleted)
	}
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted' in output, got: %q", out)
	}
}

func TestCommentRm_All_ForceSkipsPrompt(t *testing.T) {
	var deleted []string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results":     []todoist.Comment{{ID: "a1", Content: "gone", PostedAt: "2026-06-04T12:00:00.000000Z"}},
			"next_cursor": nil,
		})
	})
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/comments/"))
			w.WriteHeader(http.StatusNoContent)
		}
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Force", "p1", "")

	// No setStdin — -f must not read from stdin.
	if _, err := runCmd(t, "comment", "rm", "--all", "-f", "Force"); err != nil {
		t.Fatalf("comment rm --all -f: %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("expected the comment deleted without a prompt, got %v", deleted)
	}
}

func TestCommentRm_All_NoComments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"results": []todoist.Comment{}, "next_cursor": nil})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-abc", "Empty", "p1", "")

	out, err := runCmd(t, "comment", "rm", "--all", "Empty")
	if err != nil {
		t.Fatalf("comment rm --all (empty): %v", err)
	}
	if !strings.Contains(out, "no comments") {
		t.Errorf("expected 'no comments' message, got: %q", out)
	}
}

func TestCommentShow_PrintsFullBody(t *testing.T) {
	var deleted []string
	newTestEnv(t, commentByIDStub(t, &deleted))

	out, err := runCmd(t, "comment", "show", "cmt-1")
	if err != nil {
		t.Fatalf("comment show: %v", err)
	}
	// Full body, including lines that `ls`/`rm` would truncate.
	if !strings.Contains(out, "cmt-1 body") || !strings.Contains(out, "second line") {
		t.Errorf("expected full multi-line body, got: %q", out)
	}
	if len(deleted) != 0 {
		t.Errorf("comment show must not delete anything, got %v", deleted)
	}
}

func TestCommentRm_APIError_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"Invalid argument value"}`, http.StatusBadRequest)
	})
	newTestEnv(t, mux)

	_, err := runCmd(t, "comment", "rm", "bad-id")
	if err == nil {
		t.Fatal("expected error when API rejects the comment ID, got nil")
	}
}

func TestComment_ResolvesTaskByPrefix(t *testing.T) {
	var gotTaskID string
	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		gotTaskID = body["task_id"]
		writeJSON(w, todoist.Comment{ID: "c1", TaskID: gotTaskID, Content: "done"})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "task-full-id", "Refactor auth", "p1", "")

	if _, err := runCmd(t, "comment", "Refactor auth", "done"); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if gotTaskID != "task-full-id" {
		t.Errorf("expected full task ID resolved from name, got %q", gotTaskID)
	}
}
