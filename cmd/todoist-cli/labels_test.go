package main

import (
	"strings"
	"testing"
)

func TestLabels_ListsAll(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedLabel(t, env.conn, "l1", "urgent", 0)
	hSeedLabel(t, env.conn, "l2", "work", 1)

	out, err := runCmd(t, "labels")
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if !strings.Contains(out, "l1\turgent") {
		t.Errorf("expected 'l1\\turgent' in output, got: %q", out)
	}
	if !strings.Contains(out, "l2\twork") {
		t.Errorf("expected 'l2\\twork' in output, got: %q", out)
	}
}

func TestLabels_EmptyDB(t *testing.T) {
	newTestEnv(t, nil)

	out, err := runCmd(t, "labels")
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for no labels, got: %q", out)
	}
}

func TestLabels_OrderedByOrd(t *testing.T) {
	env := newTestEnv(t, nil)
	// Insert in reverse order — output must follow ord, not insertion order.
	hSeedLabel(t, env.conn, "l3", "personal", 2)
	hSeedLabel(t, env.conn, "l1", "urgent", 0)
	hSeedLabel(t, env.conn, "l2", "work", 1)

	out, err := runCmd(t, "labels")
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 labels, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "l1") {
		t.Errorf("expected urgent (ord=0) first, got: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "l2") {
		t.Errorf("expected work (ord=1) second, got: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "l3") {
		t.Errorf("expected personal (ord=2) third, got: %q", lines[2])
	}
}

func TestLabels_OutputIsTabSeparated(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedLabel(t, env.conn, "lx", "focus", 0)

	out, err := runCmd(t, "labels")
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if !strings.Contains(out, "\t") {
		t.Errorf("expected tab-separated output, got: %q", out)
	}
}
