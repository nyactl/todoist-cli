package todoist

type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	ProjectID   string   `json:"project_id"`
	SectionID   string   `json:"section_id"`
	ParentID    string   `json:"parent_id"`
	Labels      []string `json:"labels"`
	Priority    int      `json:"priority"`
	Order       int      `json:"order"`
	IsCompleted bool     `json:"is_completed"`
	Due         *Due     `json:"due"`
	URL         string   `json:"url"`
	CreatedAt   string   `json:"created_at"`
}

type Due struct {
	Date        string `json:"date"`
	Datetime    string `json:"datetime"`
	String      string `json:"string"`
	IsRecurring bool   `json:"is_recurring"`
	Timezone    string `json:"timezone"`
}

type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	ParentID   string `json:"parent_id"`
	Order      int    `json:"order"`
	IsArchived bool   `json:"is_archived"`
	IsFavorite bool   `json:"is_favorite"`
}

type Label struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Order      int    `json:"order"`
	IsFavorite bool   `json:"is_favorite"`
}

type Section struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
	Order     int    `json:"order"`
}

type CreateTaskRequest struct {
	Content     string   `json:"content"`
	ProjectID   string   `json:"project_id,omitempty"`
	SectionID   string   `json:"section_id,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	DueString   string   `json:"due_string,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ResultPage[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}
