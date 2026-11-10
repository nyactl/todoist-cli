package todoist

import (
	"encoding/json"
	"time"
)

// Priority values as returned by the Todoist API.
// Note: API priority 4 = p1 (urgent) in the Todoist UI; API 1 = p4 (normal).
const (
	PriorityNormal = 1
	PriorityMedium = 2
	PriorityHigh   = 3
	PriorityUrgent = 4
)

type Task struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	IsCompleted bool      `json:"checked"`
	Priority    int       `json:"priority"`
	ProjectID   string    `json:"project_id"`
	SectionID   string    `json:"section_id"`
	ParentID    string    `json:"parent_id"`
	Labels      []string  `json:"labels"`
	Due         *Due      `json:"due"`
	Deadline    *Deadline `json:"deadline"`
	Order       int       `json:"child_order"`
	CreatedAt   string    `json:"added_at"`
	// URL is not part of the API object; it is derived from the ID on decode.
	URL          string `json:"url,omitempty"`
	CommentCount int    `json:"note_count"`
}

// UnmarshalJSON fills URL, which the API does not send, from the task ID.
func (t *Task) UnmarshalJSON(b []byte) error {
	type raw Task
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*t = Task(r)
	if t.URL == "" && t.ID != "" {
		t.URL = TaskURL(t.ID)
	}
	return nil
}

// TaskURL is the task's address in the Todoist web app.
func TaskURL(id string) string {
	return "https://app.todoist.com/app/task/" + id
}

// Due is a task's due date. Date is always the local calendar day
// (YYYY-MM-DD); Datetime carries the full value when the task is due at a
// time of day, either floating ("2026-11-09T15:00:00") or fixed to a
// timezone and expressed in UTC ("2026-11-09T14:00:00.000000Z").
type Due struct {
	Date        string `json:"date"`
	Datetime    string `json:"datetime,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	IsRecurring bool   `json:"is_recurring"`
	String      string `json:"string"`
}

// UnmarshalJSON splits the API's single "date" field, which holds the time
// as well when there is one, into Date and Datetime.
func (d *Due) UnmarshalJSON(b []byte) error {
	type raw Due
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*d = Due(r)
	if d.Datetime == "" && len(d.Date) > len("2006-01-02") {
		d.Datetime = d.Date
		d.Date = LocalDate(d.Datetime)
	}
	return nil
}

// LocalDate returns the local calendar day of a due datetime. A floating
// time is already local; a fixed-timezone time is in UTC and converted.
func LocalDate(datetime string) string {
	if t, err := time.Parse(time.RFC3339Nano, datetime); err == nil {
		return t.Local().Format("2006-01-02")
	}
	return datetime[:len("2006-01-02")]
}

// LocalClock returns the local time of day (HH:MM) of a due datetime, or ""
// when it cannot be parsed.
func LocalClock(datetime string) string {
	if t, err := time.Parse(time.RFC3339Nano, datetime); err == nil {
		return t.Local().Format("15:04")
	}
	if t, err := time.Parse("2006-01-02T15:04:05", datetime); err == nil {
		return t.Format("15:04")
	}
	return ""
}

// Deadline is Todoist's hard-cutoff date, distinct from Due. Date-only.
type Deadline struct {
	Date string `json:"date"`
	Lang string `json:"lang,omitempty"`
}

type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Order      int    `json:"child_order"`
	ParentID   string `json:"parent_id"`
	IsFavorite bool   `json:"is_favorite"`
	IsArchived bool   `json:"is_archived"`
	ViewStyle  string `json:"view_style"`
}

type Label struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Order      int    `json:"order"`
	IsFavorite bool   `json:"is_favorite"`
}

type Section struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProjectID  string `json:"project_id"`
	Order      int    `json:"section_order"`
	IsArchived bool   `json:"is_archived"`
}

type Comment struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Content   string `json:"content"`
	PostedAt  string `json:"posted_at"`
	PostedUID string `json:"posted_uid"`
}

// Collaborator is a member of a project; used to resolve a comment's posted_uid
// to a display name.
type Collaborator struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateProjectRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
}

type UpdateProjectRequest struct {
	Name string `json:"name,omitempty"`
}

type UpdateLabelRequest struct {
	Name string `json:"name,omitempty"`
}

type CreateTaskRequest struct {
	Content      string   `json:"content"`
	Description  string   `json:"description,omitempty"`
	ProjectID    string   `json:"project_id,omitempty"`
	SectionID    string   `json:"section_id,omitempty"`
	ParentID     string   `json:"parent_id,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	Priority     int      `json:"priority,omitempty"`
	DueString    string   `json:"due_string,omitempty"`
	DueDate      string   `json:"due_date,omitempty"`
	DueDatetime  string   `json:"due_datetime,omitempty"`
	DeadlineDate string   `json:"deadline_date,omitempty"`
	Order        int      `json:"order,omitempty"`
}

// CompletedTask is a task as the completed-tasks endpoint returns it.
type CompletedTask struct {
	TaskID      string   `json:"id"`
	Content     string   `json:"content"`
	ProjectID   string   `json:"project_id"`
	SectionID   string   `json:"section_id"`
	ParentID    string   `json:"parent_id"`
	Labels      []string `json:"labels"`
	CompletedAt string   `json:"completed_at"`
}
