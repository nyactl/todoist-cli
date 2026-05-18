package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/todoist"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// testEnv holds an isolated DB + optional stub HTTP server for one test.
type testEnv struct {
	conn *sql.DB
	srv  *httptest.Server
}

// newTestEnv wires up an isolated temp DB and points all env vars at it.
// Pass nil for handler if the test doesn't make API calls.
func newTestEnv(t *testing.T, handler http.Handler) *testEnv {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("TODOIST_TOKEN", "test-token")

	var srv *httptest.Server
	if handler != nil {
		srv = httptest.NewServer(handler)
		t.Cleanup(srv.Close)
		t.Setenv("TODOIST_API_BASE", srv.URL)
	}

	conn, err := db.Open()
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &testEnv{conn: conn, srv: srv}
}

// resetFlags resets every flag on every command to its default value and clears
// the Changed tracking. Without this, pflag state bleeds between Execute() calls.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// runCmd executes the CLI with the given args and returns captured stdout.
// Flag state is reset before each call so tests don't bleed into each other.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	resetFlags(root)
	// StringArray flags append on each Set call — pflag can't reset them via DefValue.
	lsLabels = nil
	addLabels = nil
	editLabels = nil

	// Capture os.Stdout — most print helpers write directly to it.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	root.SetArgs(args)
	root.SilenceErrors = true
	root.SilenceUsage = true
	cmdErr := root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	return buf.String(), cmdErr
}

// --- DB seed helpers ---

func hSeedProject(t *testing.T, conn *sql.DB, id, name string) {
	t.Helper()
	if _, err := conn.ExecContext(context.Background(),
		`INSERT INTO projects (id, name) VALUES (?, ?)`, id, name); err != nil {
		t.Fatalf("seedProject: %v", err)
	}
}

func hSeedSection(t *testing.T, conn *sql.DB, id, name, projectID string, ord int) {
	t.Helper()
	if _, err := conn.ExecContext(context.Background(),
		`INSERT INTO sections (id, name, project_id, ord) VALUES (?, ?, ?, ?)`,
		id, name, projectID, ord); err != nil {
		t.Fatalf("seedSection: %v", err)
	}
}

func hSeedTask(t *testing.T, conn *sql.DB, id, content, projectID, sectionID string) {
	t.Helper()
	var sec any
	if sectionID != "" {
		sec = sectionID
	}
	if _, err := conn.ExecContext(context.Background(),
		`INSERT INTO tasks (id, content, project_id, section_id) VALUES (?, ?, ?, ?)`,
		id, content, projectID, sec); err != nil {
		t.Fatalf("seedTask: %v", err)
	}
}

// --- API stub helpers ---

type apiPage[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func pageResponse[T any](items []T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, apiPage[T]{Results: items})
	}
}

// emptyAPI returns a stub that responds to all sync endpoints with empty lists.
func emptyAPI() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{}))
	return mux
}

// noopAPI returns 204 No Content for any request — useful for close/delete/move.
func noopHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
