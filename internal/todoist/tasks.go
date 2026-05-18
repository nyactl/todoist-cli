package todoist

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetTasks(ctx context.Context, projectID string) ([]Task, error) {
	params := url.Values{}
	if projectID != "" {
		params.Set("project_id", projectID)
	}
	return queryAll[Task](c, ctx, "/tasks", params)
}

func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	var t Task
	return &t, c.doJSON(ctx, "GET", "/tasks/"+id, nil, &t)
}

func (c *Client) CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	var t Task
	return &t, c.doJSON(ctx, "POST", "/tasks", req, &t)
}

func (c *Client) UpdateTask(ctx context.Context, id string, req UpdateTaskRequest) (*Task, error) {
	var t Task
	return &t, c.doJSON(ctx, "POST", "/tasks/"+id, req, &t)
}

func (c *Client) CloseTask(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "POST", fmt.Sprintf("/tasks/%s/close", id), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) ReopenTask(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "POST", fmt.Sprintf("/tasks/%s/reopen", id), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) DeleteTask(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/tasks/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) MoveTaskToSection(ctx context.Context, taskID, sectionID string) error {
	body := map[string]string{"section_id": sectionID}
	resp, err := c.do(ctx, "POST", "/tasks/"+taskID+"/move", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) MoveTaskToProject(ctx context.Context, taskID, projectID string) error {
	body := map[string]string{"project_id": projectID}
	resp, err := c.do(ctx, "POST", "/tasks/"+taskID+"/move", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// UpdateTaskFields updates only the fields present in the map.
// Use this instead of UpdateTask when you need to send an explicit empty string
// (e.g. due_string: "" to clear a due date), which omitempty would otherwise drop.
func (c *Client) UpdateTaskFields(ctx context.Context, id string, fields map[string]any) (*Task, error) {
	var t Task
	return &t, c.doJSON(ctx, "POST", "/tasks/"+id, fields, &t)
}
