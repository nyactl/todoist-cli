package main

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

func TestCd_SetsProjectContext(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Work")

	if _, err := runCmd(t, "cd", "Work"); err != nil {
		t.Fatalf("cd: %v", err)
	}

	st, err := state.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.ProjectID != "p1" {
		t.Errorf("expected ProjectID 'p1', got %q", st.ProjectID)
	}
	if st.ProjectName != "Work" {
		t.Errorf("expected ProjectName 'Work', got %q", st.ProjectName)
	}
}

func TestCd_ClearsContext(t *testing.T) {
	newTestEnv(t, nil)
	// pre-set a context
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := runCmd(t, "cd"); err != nil {
		t.Fatalf("cd (clear): %v", err)
	}

	st, err := state.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.HasProject() {
		t.Errorf("expected empty context after 'cd', got %+v", st)
	}
}

func TestCd_UnknownProject_Errors(t *testing.T) {
	newTestEnv(t, nil)

	_, err := runCmd(t, "cd", "NoSuchProject")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
}

func TestContext_PrintsActiveProject(t *testing.T) {
	newTestEnv(t, nil)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	out, err := runCmd(t, "context")
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if !strings.Contains(out, "p1") {
		t.Errorf("expected project ID in output, got: %q", out)
	}
	if !strings.Contains(out, "Work") {
		t.Errorf("expected project name in output, got: %q", out)
	}
}

func TestContext_EmptyWhenNoContext(t *testing.T) {
	newTestEnv(t, nil)

	out, err := runCmd(t, "context")
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output with no context, got: %q", out)
	}
}

func TestCdThenContext_RoundTrip(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p42", "Personal")

	if _, err := runCmd(t, "cd", "Personal"); err != nil {
		t.Fatalf("cd: %v", err)
	}

	out, err := runCmd(t, "context")
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if !strings.Contains(out, "Personal") {
		t.Errorf("expected 'Personal' in context output, got: %q", out)
	}
}
