package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func hSeedLabeledTask(t *testing.T, env *testEnv, taskID, labelName string) {
	t.Helper()
	hSeedTask(t, env.conn, taskID, "task "+taskID, "p1", "")
	if _, err := env.conn.ExecContext(context.Background(),
		`INSERT INTO task_labels (task_id, label_name) VALUES (?, ?)`, taskID, labelName); err != nil {
		t.Fatalf("seed task_labels: %v", err)
	}
}

func labelCount(t *testing.T, env *testEnv, name string) (labels, refs int) {
	t.Helper()
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM labels WHERE name = ?`, name).Scan(&labels)
	env.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM task_labels WHERE label_name = ?`, name).Scan(&refs)
	return
}

// labelCalls records which endpoint family the command hit, so tests can assert
// personal (/labels/{id}) vs shared (/labels/shared/*) routing.
type labelCalls struct {
	personalDelete bool
	personalRename bool
	sharedRemove   bool
	sharedRename   bool
}

func labelMutateStub(c *labelCalls) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels/shared/remove", func(w http.ResponseWriter, r *http.Request) {
		c.sharedRemove = true
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/labels/shared/rename", func(w http.ResponseWriter, r *http.Request) {
		c.sharedRename = true
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/labels/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			c.personalDelete = true
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			c.personalRename = true
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			writeJSON(w, todoist.Label{ID: "l1", Name: body["name"]})
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

// --- labels rm: personal ---

func TestLabelsRm_Personal_ConfirmDeletes(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "urgent", 0)
	hSeedLabeledTask(t, env, "t1", "urgent")
	setStdin(t, "y\n")

	out, err := runCmd(t, "labels", "rm", "urgent")
	if err != nil {
		t.Fatalf("labels rm: %v", err)
	}
	if !strings.Contains(out, "on 1 task") {
		t.Errorf("expected blast-radius count in prompt, got: %q", out)
	}
	if !c.personalDelete || c.sharedRemove {
		t.Errorf("expected personal delete route, got %+v", c)
	}
	if l, r := labelCount(t, env, "urgent"); l != 0 || r != 0 {
		t.Errorf("expected label + refs removed, got labels=%d refs=%d", l, r)
	}
}

func TestLabelsRm_Personal_DeclineIsNoop(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "urgent", 0)
	hSeedLabeledTask(t, env, "t1", "urgent")
	setStdin(t, "n\n")

	out, err := runCmd(t, "labels", "rm", "urgent")
	if err != nil {
		t.Fatalf("labels rm: %v", err)
	}
	if c.personalDelete || c.sharedRemove {
		t.Error("expected no delete call when declined")
	}
	if !strings.Contains(out, "skipped: urgent") {
		t.Errorf("expected 'skipped' output, got: %q", out)
	}
	if l, _ := labelCount(t, env, "urgent"); l != 1 {
		t.Error("expected label to remain when declined")
	}
}

func TestLabelsRm_Personal_Force(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "urgent", 0)
	hSeedLabeledTask(t, env, "t1", "urgent")

	out, err := runCmd(t, "labels", "rm", "urgent", "-f")
	if err != nil {
		t.Fatalf("labels rm -f: %v", err)
	}
	if strings.Contains(out, "delete?") {
		t.Errorf("expected no prompt with --force, got: %q", out)
	}
	if !c.personalDelete {
		t.Error("expected personal delete with --force")
	}
}

func TestLabelsRm_Personal_UnusedSilent(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedLabel(t, env.conn, "l1", "stale", 0) // no references

	out, err := runCmd(t, "labels", "rm", "stale")
	if err != nil {
		t.Fatalf("labels rm: %v", err)
	}
	if strings.Contains(out, "delete?") {
		t.Errorf("a zero-task label must delete without a prompt, got: %q", out)
	}
	if !c.personalDelete {
		t.Error("expected personal delete for unused label")
	}
	if l, _ := labelCount(t, env, "stale"); l != 0 {
		t.Error("expected unused label deleted")
	}
}

func TestLabelsRm_Unused_Flag(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "used", 0)
	hSeedLabel(t, env.conn, "l2", "orphan1", 1)
	hSeedLabel(t, env.conn, "l3", "orphan2", 2)
	hSeedLabeledTask(t, env, "t1", "used")

	out, err := runCmd(t, "labels", "rm", "--unused")
	if err != nil {
		t.Fatalf("labels rm --unused: %v", err)
	}
	if !strings.Contains(out, "removed 2 unused") {
		t.Errorf("expected summary of 2 removed, got: %q", out)
	}
	if l, _ := labelCount(t, env, "used"); l != 1 {
		t.Error("in-use label must survive --unused")
	}
	for _, n := range []string{"orphan1", "orphan2"} {
		if l, _ := labelCount(t, env, n); l != 0 {
			t.Errorf("expected %q removed by --unused", n)
		}
	}
}

// --- labels rm: shared (CLI-created labels that live only on tasks) ---

func TestLabelsRm_Shared_ConfirmRemoves(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	// no personal label row — this is a shared label, only on a task
	hSeedLabeledTask(t, env, "t1", "wip")
	setStdin(t, "y\n")

	out, err := runCmd(t, "labels", "rm", "wip")
	if err != nil {
		t.Fatalf("labels rm (shared): %v", err)
	}
	if !c.sharedRemove || c.personalDelete {
		t.Errorf("expected shared remove route, got %+v", c)
	}
	if _, r := labelCount(t, env, "wip"); r != 0 {
		t.Error("expected shared label detached from tasks in cache")
	}
	_ = out
}

func TestLabelsRm_Shared_Force(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabeledTask(t, env, "t1", "wip")

	if _, err := runCmd(t, "labels", "rm", "wip", "-f"); err != nil {
		t.Fatalf("labels rm -f (shared): %v", err)
	}
	if !c.sharedRemove {
		t.Error("expected shared remove with --force")
	}
}

// --- labels rm: errors ---

func TestLabelsRm_UnknownLabel_Errors(t *testing.T) {
	newTestEnv(t, labelMutateStub(&labelCalls{}))
	if _, err := runCmd(t, "labels", "rm", "nope"); err == nil {
		t.Fatal("expected error for unknown label")
	}
}

func TestLabelsRm_UnusedWithNames_Errors(t *testing.T) {
	newTestEnv(t, labelMutateStub(&labelCalls{}))
	if _, err := runCmd(t, "labels", "rm", "--unused", "foo"); err == nil {
		t.Fatal("expected error combining --unused with names")
	}
}

func TestLabelsRm_APIError_LeavesCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/labels/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
	})
	env := newTestEnv(t, mux)
	hSeedLabel(t, env.conn, "l1", "stale", 0)

	if _, err := runCmd(t, "labels", "rm", "stale"); err == nil {
		t.Fatal("expected error when API delete fails")
	}
	if l, _ := labelCount(t, env, "stale"); l != 1 {
		t.Error("label must remain in cache when API delete fails")
	}
}

// --- labels rename ---

func TestLabelsRename_Personal(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "old", 0)
	hSeedLabeledTask(t, env, "t1", "old")

	out, err := runCmd(t, "labels", "rename", "old", "new")
	if err != nil {
		t.Fatalf("labels rename: %v", err)
	}
	if !c.personalRename || c.sharedRename {
		t.Errorf("expected personal rename route, got %+v", c)
	}
	if !strings.Contains(out, "renamed: old -> new") {
		t.Errorf("expected rename confirmation, got: %q", out)
	}
	if l, _ := labelCount(t, env, "new"); l != 1 {
		t.Error("expected personal label renamed to 'new'")
	}
	if l, r := labelCount(t, env, "old"); l != 0 || r != 0 {
		t.Errorf("expected old name gone from labels + task_labels, got labels=%d refs=%d", l, r)
	}
	if _, r := labelCount(t, env, "new"); r != 1 {
		t.Error("expected task reference updated to 'new'")
	}
}

func TestLabelsRename_Shared(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabeledTask(t, env, "t1", "old") // shared: only on a task

	if _, err := runCmd(t, "labels", "rename", "old", "new"); err != nil {
		t.Fatalf("labels rename (shared): %v", err)
	}
	if !c.sharedRename || c.personalRename {
		t.Errorf("expected shared rename route, got %+v", c)
	}
	if _, r := labelCount(t, env, "new"); r != 1 {
		t.Error("expected task reference renamed to 'new'")
	}
	if _, r := labelCount(t, env, "old"); r != 0 {
		t.Error("expected old task reference gone")
	}
}

func TestLabelsRename_CollisionPersonal_Errors(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedLabel(t, env.conn, "l1", "old", 0)
	hSeedLabel(t, env.conn, "l2", "taken", 1)

	if _, err := runCmd(t, "labels", "rename", "old", "taken"); err == nil {
		t.Fatal("expected error renaming to an existing personal label")
	}
	if c.personalRename || c.sharedRename {
		t.Error("expected no API call on collision")
	}
}

func TestLabelsRename_CollisionShared_Errors(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedProject(t, env.conn, "p1", "Work")
	hSeedLabel(t, env.conn, "l1", "old", 0)
	hSeedLabeledTask(t, env, "t1", "taken") // 'taken' exists on a task

	if _, err := runCmd(t, "labels", "rename", "old", "taken"); err == nil {
		t.Fatal("expected error renaming to a name already used on tasks")
	}
	if c.personalRename || c.sharedRename {
		t.Error("expected no API call on collision")
	}
}

func TestLabelsRename_UnknownOld_Errors(t *testing.T) {
	newTestEnv(t, labelMutateStub(&labelCalls{}))
	if _, err := runCmd(t, "labels", "rename", "ghost", "new"); err == nil {
		t.Fatal("expected error renaming an unknown label")
	}
}

func TestLabelsRename_SameName_Errors(t *testing.T) {
	env := newTestEnv(t, labelMutateStub(&labelCalls{}))
	hSeedLabel(t, env.conn, "l1", "same", 0)
	if _, err := runCmd(t, "labels", "rename", "same", "same"); err == nil {
		t.Fatal("expected error when old and new names match")
	}
}

func TestLabelsRename_MvAliasWorks(t *testing.T) {
	var c labelCalls
	env := newTestEnv(t, labelMutateStub(&c))
	hSeedLabel(t, env.conn, "l1", "old", 0)

	if _, err := runCmd(t, "labels", "mv", "old", "new"); err != nil {
		t.Fatalf("labels mv (alias of rename): %v", err)
	}
	if !c.personalRename {
		t.Error("expected rename via 'mv' alias")
	}
}
