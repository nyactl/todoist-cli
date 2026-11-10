package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// shrinkingAPI serves two projects, labels and sections on the first sync
// and only the first of each afterwards, like an account where the second
// was deleted or archived from another client.
func shrinkingAPI(calls *atomic.Int32) http.Handler {
	mux := http.NewServeMux()
	keep := func() bool { return calls.Load() > 0 }
	empty := func() bool { return calls.Load() > 1 }
	mux.HandleFunc("/labels", func(w http.ResponseWriter, r *http.Request) {
		items := []todoist.Label{{ID: "l1", Name: "keep"}, {ID: "l2", Name: "gone"}}
		if keep() {
			items = items[:1]
		}
		if empty() {
			items = nil
		}
		writeJSON(w, apiPage[todoist.Label]{Results: items})
	})
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		items := []todoist.Project{{ID: "p1", Name: "Work"}, {ID: "p2", Name: "Old"}}
		if keep() {
			items = items[:1]
		}
		if empty() {
			items = nil
		}
		writeJSON(w, apiPage[todoist.Project]{Results: items})
	})
	mux.HandleFunc("/sections", func(w http.ResponseWriter, r *http.Request) {
		items := []todoist.Section{{ID: "s1", Name: "Todo", ProjectID: "p1"}, {ID: "s2", Name: "Later", ProjectID: "p1"}}
		if keep() {
			items = items[:1]
		}
		if empty() {
			items = nil
		}
		writeJSON(w, apiPage[todoist.Section]{Results: items})
	})
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		items := []todoist.Task{{ID: "t1", Content: "Stay", ProjectID: "p1", SectionID: "s1", Priority: 1}}
		if !keep() {
			items = append(items, todoist.Task{ID: "t2", Content: "In old project", ProjectID: "p2", Priority: 1})
		}
		if empty() {
			items = nil
		}
		writeJSON(w, apiPage[todoist.Task]{Results: items})
	})
	return mux
}

func countRows(t *testing.T, env *testEnv, query string) int {
	t.Helper()
	var n int
	if err := env.conn.QueryRowContext(context.Background(), query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestSync_RemovesProjectsLabelsAndSectionsGoneFromAPI(t *testing.T) {
	var calls atomic.Int32
	env := newTestEnv(t, shrinkingAPI(&calls))

	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if countRows(t, env, `SELECT COUNT(*) FROM projects`) != 2 || countRows(t, env, `SELECT COUNT(*) FROM tasks`) != 2 {
		t.Fatal("first sync should cache both projects and tasks")
	}

	calls.Add(1)
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM projects WHERE id = 'p2'`); n != 0 {
		t.Errorf("project deleted on the server is still cached")
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM labels WHERE id = 'l2'`); n != 0 {
		t.Errorf("label deleted on the server is still cached")
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM sections WHERE id = 's2'`); n != 0 {
		t.Errorf("section deleted on the server is still cached")
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM tasks WHERE id = 't2'`); n != 0 {
		t.Errorf("task of the deleted project is still cached")
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM tasks WHERE id = 't1' AND section_id = 's1'`); n != 1 {
		t.Errorf("surviving task lost its section")
	}
}

// The empty-list case takes a different path in the purge and must clear
// the cache as well.
func TestSync_RemovesEverythingWhenAPIIsEmpty(t *testing.T) {
	var calls atomic.Int32
	env := newTestEnv(t, shrinkingAPI(&calls))
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatal(err)
	}
	calls.Store(2)
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"projects", "labels", "sections", "tasks"} {
		if n := countRows(t, env, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Errorf("%s still holds %d row(s) after syncing an empty account", table, n)
		}
	}
}

// A task can still name a section the same sync no longer lists (the two are
// fetched in parallel, so one can see a change the other missed). The task is
// kept without its section instead of failing the whole sync.
func TestSync_TaskInVanishedSectionKeepsSyncing(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{{ID: "p1", Name: "Work"}}))
	mux.HandleFunc("/sections", func(w http.ResponseWriter, r *http.Request) {
		items := []todoist.Section{{ID: "s1", Name: "Todo", ProjectID: "p1"}}
		if calls.Load() > 0 {
			items = nil
		}
		writeJSON(w, apiPage[todoist.Section]{Results: items})
	})
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{{ID: "t1", Content: "Stay", ProjectID: "p1", SectionID: "s1", Priority: 1}}))
	env := newTestEnv(t, mux)

	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	calls.Add(1)
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatalf("second sync must not fail on the vanished section: %v", err)
	}
	if n := countRows(t, env, `SELECT COUNT(*) FROM tasks WHERE id = 't1' AND section_id IS NULL`); n != 1 {
		t.Errorf("task should be kept without its section")
	}
}
