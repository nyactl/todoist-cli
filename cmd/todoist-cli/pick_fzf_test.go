//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/state"
)

// fakeFzf puts a stand-in fzf on PATH. It records its input and arguments and
// behaves according to mode: "match:<text>" selects the first line containing
// text, "cancel" exits 130 like ESC, "none" selects nothing, "fail" exits 2.
// It only uses shell builtins because PATH holds nothing else.
func fakeFzf(t *testing.T, mode string) (inputFile string) {
	t.Helper()
	dir := t.TempDir()
	inputFile = filepath.Join(dir, "input")
	script := `#!/bin/sh
printf '%s\n' "$*" > "` + inputFile + `.args"
: > "` + inputFile + `"
picked=""
while IFS= read -r line; do
	printf '%s\n' "$line" >> "` + inputFile + `"
	case "$FAKE_FZF_MODE" in
	match:*) want="${FAKE_FZF_MODE#match:}"
		case "$line" in *"$want"*) [ -z "$picked" ] && picked="$line" ;; esac ;;
	esac
done
case "$FAKE_FZF_MODE" in
cancel) exit 130 ;;
fail) exit 2 ;;
none) exit 0 ;;
esac
printf '%s\n' "$picked"
`
	if err := os.WriteFile(filepath.Join(dir, "fzf"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FAKE_FZF_MODE", mode)
	return inputFile
}

func seedPickTasks(t *testing.T, env *testEnv) {
	t.Helper()
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedProject(t, env.conn, "p2", "Home")
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, priority, due_date) VALUES ('t-today', 'Pay invoice', 'p1', 4, date('now', 'localtime'))`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id) VALUES ('t-undated', 'Refactor parser', 'p1')`)
	env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t-home', 'Water plants', 'p2', date('now', 'localtime'))`)
	env.conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t-undated', 'deep'), ('t-home', 'deep')`)
}

func TestPick_PrintsSelectedTaskID(t *testing.T) {
	env := newTestEnv(t, nil)
	seedPickTasks(t, env)
	input := fakeFzf(t, "match:Water plants")

	out, err := runCmd(t, "pick")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "t-home" {
		t.Errorf("pick printed %q, want t-home", out)
	}
	fed, _ := os.ReadFile(input)
	if !strings.Contains(string(fed), "t-today\t!!Pay invoice\tWork\ttoday") {
		t.Errorf("fzf input missing formatted due-today task:\n%s", fed)
	}
	if strings.Contains(string(fed), "Refactor parser") {
		t.Errorf("without a context pick should offer only due tasks:\n%s", fed)
	}
	args, _ := os.ReadFile(input + ".args")
	if !strings.Contains(string(args), "--with-nth=2..") {
		t.Errorf("fzf should hide the ID column, args: %s", args)
	}
}

func TestPick_WithContextOffersAllProjectTasks(t *testing.T) {
	env := newTestEnv(t, nil)
	seedPickTasks(t, env)
	if err := state.Save(&state.State{ProjectID: "p1", ProjectName: "Work"}); err != nil {
		t.Fatal(err)
	}
	input := fakeFzf(t, "match:Refactor")

	out, err := runCmd(t, "pick")
	if err != nil || strings.TrimSpace(out) != "t-undated" {
		t.Fatalf("pick = %q, %v", out, err)
	}
	fed, _ := os.ReadFile(input)
	if strings.Contains(string(fed), "Water plants") {
		t.Errorf("tasks from other projects offered in context:\n%s", fed)
	}
}

func TestPick_LabelFilter(t *testing.T) {
	env := newTestEnv(t, nil)
	seedPickTasks(t, env)
	input := fakeFzf(t, "match:Refactor")

	out, err := runCmd(t, "pick", "-l", "deep")
	if err != nil || strings.TrimSpace(out) != "t-undated" {
		t.Fatalf("pick -l = %q, %v", out, err)
	}
	fed, _ := os.ReadFile(input)
	if strings.Contains(string(fed), "Pay invoice") || !strings.Contains(string(fed), "Water plants") {
		t.Errorf("label filter not applied:\n%s", fed)
	}
}

func TestPick_CancelOrEmptySelectionIsSilent(t *testing.T) {
	for _, mode := range []string{"cancel", "none"} {
		t.Run(mode, func(t *testing.T) {
			env := newTestEnv(t, nil)
			seedPickTasks(t, env)
			fakeFzf(t, mode)
			out, err := runCmd(t, "pick")
			if err != nil || out != "" {
				t.Errorf("pick = %q, %v; want no output and no error", out, err)
			}
		})
	}
}

func TestPick_FzfFailureIsReported(t *testing.T) {
	env := newTestEnv(t, nil)
	seedPickTasks(t, env)
	fakeFzf(t, "fail")
	if _, err := runCmd(t, "pick"); err == nil {
		t.Error("expected error when fzf exits with status 2")
	}
}
