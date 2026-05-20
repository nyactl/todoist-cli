package main

import (
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	newTestEnv(t, nil)
	out, err := runCmd(t, "--version")
	if err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if !strings.Contains(out, version) {
		t.Errorf("expected version %q in output, got: %q", version, out)
	}
}
