package todoist

// Priority values as returned by the Todoist API.
// Note: API priority 4 = p1 (urgent) in the Todoist UI; API 1 = p4 (normal).
const (
	PriorityNormal = 1
	PriorityMedium = 2
	PriorityHigh   = 3
	PriorityUrgent = 4
)

type Task struct {
	ID           string   `json:"id"`
	Content      string   `json:"content"`
	Description  string   `json:"description"`
	IsCompleted  bool     `json:"is_completed"`
	Priority     int      `json:"priority"`
	ProjectID    string   `json:"project_id"`
	SectionID    string   `json:"section_id"`
	ParentID     string   `json:"parent_id"`
	Labels       []string `json:"labels"`
	Due          *Due     `json:"due"`
	Order        int      `json:"order"`
	CreatedAt    string   `json:"created_at"`
	URL          string   `json:"url"`
	CommentCount int      `json:"comment_count"`
}

type Due struct {
	Date        string `json:"date"`
	Datetime    string `json:"datetime,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	IsRecurring bool   `json:"is_recurring"`
	String      string `json:"string"`
}

type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Order      int    `json:"order"`
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
	Order      int    `json:"order"`
	IsArchived bool   `json:"is_archived"`
}

type Comment struct {
	ID       string `json:"id"`
	TaskID   string `json:"task_id"`
	Content  string `json:"content"`
	PostedAt string `json:"posted_at"`
}

type CreateTaskRequest struct {
	Content     string   `json:"content"`
	Description string   `json:"description,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	SectionID   string   `json:"section_id,omitempty"`
	ParentID    string   `json:"parent_id,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	DueString   string   `json:"due_string,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	DueDatetime string   `json:"due_datetime,omitempty"`
	Order       int      `json:"order,omitempty"`
}

type UpdateTaskRequest struct {
	Content     string   `json:"content,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	DueString   string   `json:"due_string,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	DueDatetime string   `json:"due_datetime,omitempty"`
}

type CompletedTask struct {
	TaskID      string `json:"task_id"`
	Content     string `json:"content"`
	ProjectID   string `json:"project_id"`
	SectionID   string `json:"section_id"`
	CompletedAt string `json:"completed_at"`
}

type CompletedResult struct {
	Tasks       []CompletedTask
	ProjectName map[string]string
	SectionName map[string]string
}
