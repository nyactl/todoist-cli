package todoist

import (
	"encoding/json"
	"testing"
)

// Objects as API v1 returns them (captured from a live account).
const (
	v1Task = `{"user_id":"61160520","id":"6hx5J4wW7339h4Cf","project_id":"6hp5Xr7qXjM5WrQ7","section_id":"6hx5J4vpc6MMpPf7",
		"parent_id":null,"labels":["tdc-live-lbl"],"deadline":null,"duration":null,"is_collapsed":false,"checked":true,
		"is_deleted":false,"added_at":"2026-11-08T20:21:12.744120Z","completed_at":null,"updated_at":"2026-11-08T20:21:12.744135Z",
		"due":{"date":"2026-11-09","timezone":null,"string":"9 Nov","lang":"en","is_recurring":false},
		"priority":1,"child_order":7,"order_key":"a2F0u","content":"probe","description":"","note_count":3,"day_order":-1}`
	v1Project = `{"id":"6hp5Xr7qXjM5WrQ7","child_order":4,"order_key":"a0","color":"charcoal","is_archived":false,
		"is_favorite":false,"name":"Inbox","view_style":"list","parent_id":null,"inbox_project":true}`
	v1Section = `{"id":"6hx5J4vpc6MMpPf7","project_id":"6hp5Xr7qXjM5WrQ7","name":"Doing","section_order":2,"is_archived":false}`
)

func TestTask_DecodesAPIv1FieldNames(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(v1Task), &task); err != nil {
		t.Fatal(err)
	}
	if !task.IsCompleted {
		t.Error("checked not mapped to IsCompleted")
	}
	if task.CreatedAt != "2026-11-08T20:21:12.744120Z" {
		t.Errorf("added_at not mapped to CreatedAt: %q", task.CreatedAt)
	}
	if task.CommentCount != 3 {
		t.Errorf("note_count not mapped to CommentCount: %d", task.CommentCount)
	}
	if task.Order != 7 {
		t.Errorf("child_order not mapped to Order: %d", task.Order)
	}
	if want := "https://app.todoist.com/app/task/6hx5J4wW7339h4Cf"; task.URL != want {
		t.Errorf("URL = %q, want %q", task.URL, want)
	}
}

func TestTask_KeepsExplicitURL(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(`{"id":"x","content":"c","url":"https://example.test/x"}`), &task); err != nil {
		t.Fatal(err)
	}
	if task.URL != "https://example.test/x" {
		t.Errorf("URL = %q", task.URL)
	}
}

func TestProject_DecodesChildOrder(t *testing.T) {
	var p Project
	if err := json.Unmarshal([]byte(v1Project), &p); err != nil {
		t.Fatal(err)
	}
	if p.Order != 4 {
		t.Errorf("child_order not mapped to Order: %d", p.Order)
	}
}

func TestSection_DecodesSectionOrder(t *testing.T) {
	var s Section
	if err := json.Unmarshal([]byte(v1Section), &s); err != nil {
		t.Fatal(err)
	}
	if s.Order != 2 {
		t.Errorf("section_order not mapped to Order: %d", s.Order)
	}
}
