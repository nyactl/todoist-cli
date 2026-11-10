package main

import (
	"strings"
	"testing"
)

func TestCd_ByIDStoresProjectName(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "6hp5Xr7qXjM5WrQ7", "Work")

	if _, err := runCmd(t, "cd", "6hp5Xr7qXjM5WrQ7"); err != nil {
		t.Fatalf("cd by id: %v", err)
	}
	out, err := runCmd(t, "context")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != "6hp5Xr7qXjM5WrQ7\tWork" {
		t.Errorf("context = %q, want the ID and the project's name", got)
	}
}
