package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/todoist"
)

// withStdin feeds input to os.Stdin for the rest of the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString(input)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; r.Close() })
}

// captureStderr runs fn and returns what it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

// authAPI accepts only the given token and serves an account with one project.
func authAPI(validToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{}))
	mux.HandleFunc("/tasks", pageResponse([]todoist.Task{}))
	projects := pageResponse([]todoist.Project{{ID: "p1", Name: "Work"}})
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+validToken {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Invalid token","http_code":401}`))
			return
		}
		projects(w, r)
	})
	return mux
}

func TestAuthLogin_StoresVerifiedTokenAndSyncs(t *testing.T) {
	env := newTestEnv(t, authAPI("good-token"))
	t.Setenv("TODOIST_TOKEN", "")
	withStdin(t, "  good-token \n")

	var err error
	stderr := captureStderr(t, func() { _, err = runCmd(t, "auth", "login") })
	if err != nil {
		t.Fatalf("auth login: %v\n%s", err, stderr)
	}
	if got, _ := config.GetToken(); got != "good-token" {
		t.Errorf("stored token = %q, want trimmed good-token", got)
	}
	if !strings.Contains(stderr, "logged in — 1 projects found") {
		t.Errorf("stderr = %q", stderr)
	}
	var n int
	env.conn.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = 'p1'`).Scan(&n)
	if n != 1 {
		t.Error("login did not sync projects into the cache")
	}
}

func TestAuthLogin_RejectsInvalidTokenWithoutStoringIt(t *testing.T) {
	newTestEnv(t, authAPI("good-token"))
	t.Setenv("TODOIST_TOKEN", "")
	withStdin(t, "bad-token\n")

	var err error
	captureStderr(t, func() { _, err = runCmd(t, "auth", "login") })
	if err == nil || !strings.Contains(err.Error(), "token verification failed") {
		t.Fatalf("err = %v, want verification failure", err)
	}
	if _, err := config.GetToken(); err == nil {
		t.Error("invalid token was stored")
	}
}

func TestAuthLogin_EmptyToken(t *testing.T) {
	newTestEnv(t, authAPI("good-token"))
	withStdin(t, "\n")

	var err error
	captureStderr(t, func() { _, err = runCmd(t, "auth", "login") })
	if err == nil || !strings.Contains(err.Error(), "token cannot be empty") {
		t.Errorf("err = %v", err)
	}
}

func TestAuthLogout_RemovesToken(t *testing.T) {
	newTestEnv(t, nil)
	t.Setenv("TODOIST_TOKEN", "")
	if err := config.SetToken("to-remove"); err != nil {
		t.Fatal(err)
	}

	var err error
	stderr := captureStderr(t, func() { _, err = runCmd(t, "auth", "logout") })
	if err != nil || !strings.Contains(stderr, "logged out") {
		t.Fatalf("auth logout: %v %q", err, stderr)
	}
	if _, err := config.GetToken(); err == nil {
		t.Error("token still present after logout")
	}
}

func TestAuthLogout_NothingStored(t *testing.T) {
	newTestEnv(t, nil)
	var err error
	captureStderr(t, func() { _, err = runCmd(t, "auth", "logout") })
	if err == nil || !strings.Contains(err.Error(), "remove token") {
		t.Errorf("err = %v", err)
	}
}

func TestAuthStatus(t *testing.T) {
	newTestEnv(t, authAPI("good-token"))

	t.Setenv("TODOIST_TOKEN", "good-token")
	var err error
	stderr := captureStderr(t, func() { _, err = runCmd(t, "auth", "status") })
	if err != nil || !strings.Contains(stderr, "authenticated — 1 projects") {
		t.Errorf("valid token: %v %q", err, stderr)
	}

	t.Setenv("TODOIST_TOKEN", "bad-token")
	captureStderr(t, func() { _, err = runCmd(t, "auth", "status") })
	if err == nil || !strings.Contains(err.Error(), "token invalid") {
		t.Errorf("invalid token: err = %v", err)
	}

	t.Setenv("TODOIST_TOKEN", "")
	captureStderr(t, func() { _, err = runCmd(t, "auth", "status") })
	if err == nil || !strings.Contains(err.Error(), "auth login") {
		t.Errorf("no token: err = %v", err)
	}
}
