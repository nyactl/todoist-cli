package tasks_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func seedProject(t *testing.T, conn *sql.DB, id, name string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO projects (id, name) VALUES (?, ?)`, id, name)
	if err != nil {
		t.Fatalf("seedProject: %v", err)
	}
}

func seedSection(t *testing.T, conn *sql.DB, id, name, projectID string, ord int) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO sections (id, name, project_id, ord) VALUES (?, ?, ?, ?)`, id, name, projectID, ord)
	if err != nil {
		t.Fatalf("seedSection: %v", err)
	}
}

func seedTask(t *testing.T, conn *sql.DB, id, content, projectID, sectionID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO tasks (id, content, project_id, section_id) VALUES (?, ?, ?, ?)`,
		id, content, projectID, nullIfEmpty(sectionID),
	)
	if err != nil {
		t.Fatalf("seedTask: %v", err)
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ProjectByName

func TestProjectByName_Found(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "proj1", "Work")

	id, err := tasks.ProjectByName(context.Background(), conn, "Work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "proj1" {
		t.Errorf("got id %q, want %q", id, "proj1")
	}
}

func TestProjectByName_NotFound(t *testing.T) {
	conn := openTestDB(t)

	_, err := tasks.ProjectByName(context.Background(), conn, "NoSuchProject")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// SectionByName

func TestSectionByName_Found(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedSection(t, conn, "s1", "Backlog", "p1", 0)

	id, err := tasks.SectionByName(context.Background(), conn, "Backlog", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "s1" {
		t.Errorf("got id %q, want %q", id, "s1")
	}
}

func TestSectionByName_NotFound(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")

	_, err := tasks.SectionByName(context.Background(), conn, "NoSuchSection", "p1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSectionByName_WrongProject(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedProject(t, conn, "p2", "Personal")
	seedSection(t, conn, "s1", "Backlog", "p1", 0)

	_, err := tasks.SectionByName(context.Background(), conn, "Backlog", "p2")
	if err == nil {
		t.Fatal("expected error when section belongs to different project")
	}
}

// ByProject

func TestByProject_ReturnsTasksInProject(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedProject(t, conn, "p2", "Personal")
	seedTask(t, conn, "t1", "Task A", "p1", "")
	seedTask(t, conn, "t2", "Task B", "p1", "")
	seedTask(t, conn, "t3", "Task C", "p2", "")

	ts, err := tasks.ByProject(context.Background(), conn, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(ts))
	}
}

func TestByProject_EmptyProject(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")

	ts, err := tasks.ByProject(context.Background(), conn, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(ts))
	}
}

func TestByProject_ExcludesCompleted(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "t1", "Open", "p1", "")
	_, err := conn.Exec(`UPDATE tasks SET is_completed = 1 WHERE id = 'done1'`)
	if err != nil {
		t.Fatal(err)
	}
	conn.Exec(`INSERT INTO tasks (id, content, project_id, is_completed) VALUES ('done1', 'Done', 'p1', 1)`)

	ts, err := tasks.ByProject(context.Background(), conn, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 1 {
		t.Fatalf("expected 1 open task, got %d", len(ts))
	}
	if ts[0].ID != "t1" {
		t.Errorf("expected task t1, got %q", ts[0].ID)
	}
}

// ByID

func TestByID_ExactIDMatch(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "abc123", "Buy milk", "p1", "")

	task, err := tasks.ByID(context.Background(), conn, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Content != "Buy milk" {
		t.Errorf("got content %q", task.Content)
	}
}

func TestByID_PrefixMatch(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "abc123", "Buy milk", "p1", "")

	task, err := tasks.ByID(context.Background(), conn, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "abc123" {
		t.Errorf("got id %q", task.ID)
	}
}

func TestByID_AmbiguousPrefix(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "abc111", "Task one", "p1", "")
	seedTask(t, conn, "abc222", "Task two", "p1", "")

	_, err := tasks.ByID(context.Background(), conn, "abc")
	if err == nil {
		t.Fatal("expected ambiguous error, got nil")
	}
}

func TestByID_ContentFallback(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "xyz999", "Buy milk", "p1", "")

	task, err := tasks.ByID(context.Background(), conn, "Buy milk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "xyz999" {
		t.Errorf("got id %q", task.ID)
	}
}

func TestByID_SubstringMatch(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "xyz999", "Lampen kaufen", "p1", "")

	task, err := tasks.ByID(context.Background(), conn, "Lampen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "xyz999" {
		t.Errorf("got id %q, want xyz999", task.ID)
	}
}

func TestByID_SubstringMatch_CaseInsensitive(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "xyz999", "Lampen kaufen", "p1", "")

	task, err := tasks.ByID(context.Background(), conn, "lampen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "xyz999" {
		t.Errorf("got id %q, want xyz999", task.ID)
	}
}

func TestByID_SubstringMatch_Ambiguous(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "t1", "Buy milk today", "p1", "")
	seedTask(t, conn, "t2", "Buy milk tomorrow", "p1", "")

	_, err := tasks.ByID(context.Background(), conn, "Buy milk")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}

func TestByID_NotFound(t *testing.T) {
	conn := openTestDB(t)

	_, err := tasks.ByID(context.Background(), conn, "nosuch")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// DueToday

func TestDueToday_ReturnsPastAndTodayTasks(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")

	conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t1', 'Overdue', 'p1', date('now', '-1 day'))`)
	conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t2', 'Today', 'p1', date('now'))`)
	conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES ('t3', 'Tomorrow', 'p1', date('now', '+1 day'))`)
	conn.Exec(`INSERT INTO tasks (id, content, project_id) VALUES ('t4', 'No due', 'p1')`)

	ts, err := tasks.DueToday(context.Background(), conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 2 {
		t.Fatalf("expected 2 tasks (overdue + today), got %d", len(ts))
	}
}

// ByLabels

func TestByLabels_ANDLogic(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "t1", "Both labels", "p1", "")
	seedTask(t, conn, "t2", "One label", "p1", "")
	seedTask(t, conn, "t3", "No labels", "p1", "")

	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'work')`)
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2', 'urgent')`)

	ts, err := tasks.ByLabels(context.Background(), conn, []string{"urgent", "work"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 1 {
		t.Fatalf("expected 1 task with both labels, got %d", len(ts))
	}
	if ts[0].ID != "t1" {
		t.Errorf("expected t1, got %q", ts[0].ID)
	}
}

// Subtasks

func TestSubtasks_ReturnsChildren(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "parent", "Parent task", "p1", "")
	conn.Exec(`INSERT INTO tasks (id, content, project_id, parent_id) VALUES ('child1', 'Child A', 'p1', 'parent')`)
	conn.Exec(`INSERT INTO tasks (id, content, project_id, parent_id) VALUES ('child2', 'Child B', 'p1', 'parent')`)

	subs, err := tasks.Subtasks(context.Background(), conn, "parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subs))
	}
}

// ProjectExists

func TestProjectExists_True(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")

	ok, err := tasks.ProjectExists(context.Background(), conn, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for existing project")
	}
}

func TestProjectExists_False(t *testing.T) {
	conn := openTestDB(t)

	ok, err := tasks.ProjectExists(context.Background(), conn, "no-such-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for missing project")
	}
}

// ByLabels edge cases

func TestByLabels_EmptyResult(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "t1", "No labels", "p1", "")

	ts, err := tasks.ByLabels(context.Background(), conn, []string{"urgent"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(ts))
	}
}

func TestByLabels_RequiresAtLeastOneLabel(t *testing.T) {
	conn := openTestDB(t)

	_, err := tasks.ByLabels(context.Background(), conn, nil, "")
	if err == nil {
		t.Fatal("expected error for empty label list")
	}
}

func TestByLabels_FilteredByProject(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedProject(t, conn, "p2", "Personal")
	seedTask(t, conn, "t1", "Work task", "p1", "")
	seedTask(t, conn, "t2", "Personal task", "p2", "")
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t2', 'urgent')`)

	ts, err := tasks.ByLabels(context.Background(), conn, []string{"urgent"}, "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ts) != 1 || ts[0].ID != "t1" {
		t.Fatalf("expected only t1 in p1, got %v", ts)
	}
}

// Labels

func TestLabels_ReturnsTaskLabels(t *testing.T) {
	conn := openTestDB(t)
	seedProject(t, conn, "p1", "Work")
	seedTask(t, conn, "t1", "Task", "p1", "")
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'urgent')`)
	conn.Exec(`INSERT INTO task_labels (task_id, label_name) VALUES ('t1', 'work')`)

	lbls, err := tasks.Labels(context.Background(), conn, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lbls) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(lbls))
	}
}
