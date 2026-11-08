package main

import (
	"testing"

	"github.com/nyactl/todoist-cli/internal/config"
)

// On a terminal the token must be read without echo; piped input (scripts,
// the other auth tests) keeps the plain line read.
func TestAuthLogin_TerminalReadsTokenWithoutEcho(t *testing.T) {
	newTestEnv(t, authAPI("tty-token"))
	t.Setenv("TODOIST_TOKEN", "")
	withStdin(t, "echoed-token\n")

	oldIsTerminal, oldReadPassword := stdinIsTerminal, readPassword
	t.Cleanup(func() { stdinIsTerminal, readPassword = oldIsTerminal, oldReadPassword })
	stdinIsTerminal = func() bool { return true }
	called := false
	readPassword = func() ([]byte, error) {
		called = true
		return []byte(" tty-token "), nil
	}

	var err error
	stderr := captureStderr(t, func() { _, err = runCmd(t, "auth", "login") })
	if err != nil {
		t.Fatalf("auth login: %v\n%s", err, stderr)
	}
	if !called {
		t.Error("token was not read with the no-echo reader on a terminal")
	}
	if got, _ := config.GetToken(); got != "tty-token" {
		t.Errorf("stored token = %q, want tty-token", got)
	}
}
