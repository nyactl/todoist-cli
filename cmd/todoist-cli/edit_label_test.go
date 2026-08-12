package main

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestApplyLabelDelta(t *testing.T) {
	tests := []struct {
		name             string
		current, add, rm []string
		want             []string
	}{
		{"add new", []string{"a", "b"}, []string{"c"}, nil, []string{"a", "b", "c"}},
		{"add existing is noop", []string{"a", "b"}, []string{"a"}, nil, []string{"a", "b"}},
		{"remove existing", []string{"a", "b"}, nil, []string{"a"}, []string{"b"}},
		{"remove absent is noop", []string{"a", "b"}, nil, []string{"z"}, []string{"a", "b"}},
		{"add wins over remove", []string{"a"}, []string{"x"}, []string{"x"}, []string{"a", "x"}},
		{"remove all clears", []string{"a", "b"}, nil, []string{"a", "b"}, []string{}},
		{"output is sorted", []string{"b", "a"}, []string{"c"}, nil, []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyLabelDelta(tc.current, tc.add, tc.rm)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("applyLabelDelta(%v, add=%v, rm=%v) = %v, want %v",
					tc.current, tc.add, tc.rm, got, tc.want)
			}
		})
	}
}

// captureLabelsEnv seeds a task t1 with labels kenji, nao and returns a mux that
// records the labels sent to the update endpoint.
func captureLabelsEnv(t *testing.T, gotLabels *[]string) *testEnv {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Labels []string `json:"labels"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		*gotLabels = body.Labels
		writeJSON(w, todoist.Task{ID: "t1", Content: "Task", Labels: body.Labels})
	})
	env := newTestEnv(t, mux)
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedTask(t, env.conn, "t1", "Task", "p1", "")
	if _, err := env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES ('t1','kenji'),('t1','nao')`); err != nil {
		t.Fatalf("seed labels: %v", err)
	}
	return env
}

func TestEdit_AddLabel_KeepsExisting(t *testing.T) {
	var got []string
	env := captureLabelsEnv(t, &got)

	if _, err := runCmd(t, "edit", "t1", "--add-label", "wip"); err != nil {
		t.Fatalf("edit --add-label: %v", err)
	}
	if want := []string{"kenji", "nao", "wip"}; !reflect.DeepEqual(sorted(got), want) {
		t.Errorf("labels sent = %v, want %v", got, want)
	}
	// cache reflects the new set
	var n int
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM task_labels WHERE task_id='t1' AND label_name='wip'`).Scan(&n)
	if n != 1 {
		t.Error("expected wip label persisted in cache")
	}
}

func TestEdit_RemoveLabel_KeepsRest(t *testing.T) {
	var got []string
	captureLabelsEnv(t, &got)

	if _, err := runCmd(t, "edit", "t1", "--remove-label", "kenji"); err != nil {
		t.Fatalf("edit --remove-label: %v", err)
	}
	if want := []string{"nao"}; !reflect.DeepEqual(sorted(got), want) {
		t.Errorf("labels sent = %v, want %v", got, want)
	}
}

func TestEdit_RemoveAbsentLabel_IsNoop(t *testing.T) {
	var got []string
	captureLabelsEnv(t, &got)

	if _, err := runCmd(t, "edit", "t1", "--remove-label", "ghost"); err != nil {
		t.Fatalf("edit --remove-label absent: %v", err)
	}
	if want := []string{"kenji", "nao"}; !reflect.DeepEqual(sorted(got), want) {
		t.Errorf("labels sent = %v, want %v (unchanged set expected)", got, want)
	}
}

func TestEdit_AddAndRemoveTogether(t *testing.T) {
	var got []string
	captureLabelsEnv(t, &got)

	if _, err := runCmd(t, "edit", "t1", "--remove-label", "kenji", "--add-label", "wip"); err != nil {
		t.Fatalf("edit combined: %v", err)
	}
	if want := []string{"nao", "wip"}; !reflect.DeepEqual(sorted(got), want) {
		t.Errorf("labels sent = %v, want %v", got, want)
	}
}

func TestEdit_LabelAndAddLabel_MutuallyExclusive(t *testing.T) {
	var got []string
	captureLabelsEnv(t, &got)

	_, err := runCmd(t, "edit", "t1", "-l", "x", "--add-label", "y")
	if err == nil {
		t.Fatal("expected error combining --label with --add-label, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("expected mutual-exclusion error, got: %v", err)
	}
	if got != nil {
		t.Errorf("no update should have been sent, but labels were: %v", got)
	}
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}
