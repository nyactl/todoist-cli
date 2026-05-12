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
