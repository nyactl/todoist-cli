package main

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const completedPath = "/tasks/completed/by_completion_date"

// completedWindowAPI emulates the real endpoint: it requires since and until,
// rejects windows over three months like the API does, and records each
// requested window.
func completedWindowAPI(t *testing.T, windows *[][2]time.Time, mu *sync.Mutex, body string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(completedPath, func(w http.ResponseWriter, r *http.Request) {
		since, err1 := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
		until, err2 := time.Parse(time.RFC3339, r.URL.Query().Get("until"))
		if err1 != nil || err2 != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"since and until are required","error_code":20}`))
			return
		}
		if until.After(since.AddDate(0, 3, 0)) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"completion date range must not exceed 3 months","error_code":20}`))
			return
		}
		mu.Lock()
		*windows = append(*windows, [2]time.Time{since, until})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	return mux
}

func TestLsDone_YearSpansSeveralWindows(t *testing.T) {
	var windows [][2]time.Time
	var mu sync.Mutex
	newTestEnv(t, completedWindowAPI(t, &windows, &mu, `{"items":[],"next_cursor":null}`))

	if _, err := runCmd(t, "ls", "--done", "year"); err != nil {
		t.Fatalf("ls --done year: %v", err)
	}
	if len(windows) < 4 {
		t.Fatalf("a year needs at least four 3-month windows, got %d", len(windows))
	}
	start, _ := parseSince("year")
	earliest, latest := windows[0][0], windows[0][1]
	for _, w := range windows {
		if w[0].Before(earliest) {
			earliest = w[0]
		}
		if w[1].After(latest) {
			latest = w[1]
		}
	}
	if !earliest.Equal(start.UTC()) {
		t.Errorf("earliest since = %v, want %v", earliest, start.UTC())
	}
	if latest.Before(time.Now().Add(-time.Minute)) {
		t.Errorf("latest until = %v, should reach now", latest)
	}
}

func TestLsDone_FollowsCursor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(completedPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			w.Write([]byte(`{"items":[{"id":"pg1aaaa","content":"First page","project_id":"p1","labels":[],"completed_at":"2026-08-12T09:00:00Z"}],"next_cursor":"next"}`))
			return
		}
		w.Write([]byte(`{"items":[{"id":"pg2bbbb","content":"Second page","project_id":"p1","labels":[],"completed_at":"2026-08-12T08:00:00Z"}],"next_cursor":null}`))
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")

	out, err := runCmd(t, "ls", "--done", "week")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "First page") || !strings.Contains(out, "Second page") {
		t.Errorf("second page of completed tasks missing:\n%s", out)
	}
}

// Labels come from the completed task itself, so the filter works for tasks
// that were never cached or were purged by a sync.
func TestLsDone_LabelFilterUsesAPILabels(t *testing.T) {
	newTestEnv(t, completedHandler(
		`{"id":"lab1aaaa","content":"Write talk","project_id":"p1","labels":["deep"],"completed_at":"2026-08-12T09:00:00Z"},`+
			`{"id":"lab2bbbb","content":"Reply mail","project_id":"p1","labels":["shallow"],"completed_at":"2026-08-12T08:00:00Z"}`))

	out, err := runCmd(t, "ls", "--done", "week", "-l", "deep")
	if err != nil {
		t.Fatalf("ls --done -l deep: %v", err)
	}
	if !strings.Contains(out, "Write talk") || strings.Contains(out, "Reply mail") {
		t.Errorf("-l deep should keep only the task labelled deep:\n%s", out)
	}
	if strings.Contains(out, "@deep") {
		t.Errorf("labels must not leak into the content:\n%s", out)
	}

	if _, err := runCmd(t, "ls", "--done", "week", "-l", "typo"); err == nil {
		t.Error("an unknown label should still be reported as an error")
	}
}

func TestLsDone_NamesComeFromCache(t *testing.T) {
	env := newTestEnv(t, completedHandler(
		`{"id":"nm1aaaa","content":"Ship it","project_id":"p1","section_id":"s1","labels":[],"completed_at":"2026-08-12T09:00:00Z"}`))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedSection(t, env.conn, "s1", "Doing", "p1", 0)

	out, err := runCmd(t, "ls", "--done", "week")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "  Work\n") {
		t.Errorf("project name should resolve from the cache:\n%s", out)
	}
}
