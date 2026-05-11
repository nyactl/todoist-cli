package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.todoist.com/api/v1"

type Client struct {
	token  string
	http   *http.Client
}

func New(token string) *Client {
	return &Client{token: token, http: &http.Client{}}
}

func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	return paginate[Project](ctx, c, "/projects")
}

func (c *Client) GetLabels(ctx context.Context) ([]Label, error) {
	return paginate[Label](ctx, c, "/labels/personal")
}

func (c *Client) GetTasks(ctx context.Context, projectID string) ([]Task, error) {
	path := "/tasks"
	if projectID != "" {
		path += "?project_id=" + projectID
	}
	return paginate[Task](ctx, c, path)
}

func (c *Client) CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var t Task
	if err := c.do(ctx, http.MethodPost, "/tasks", bytes.NewReader(body), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *Client) CloseTask(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/tasks/"+id+"/close", nil, nil)
}

func (c *Client) ReopenTask(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/tasks/"+id+"/reopen", nil, nil)
}

func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	var t Task
	if err := c.do(ctx, http.MethodGet, "/tasks/"+id, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func paginate[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	cursor := ""
	for {
		sep := "?"
		if len(path) > 0 && contains(path, '?') {
			sep = "&"
		}
		url := path
		if cursor != "" {
			url = path + sep + "cursor=" + cursor
		}
		var page ResultPage[T]
		if err := c.do(ctx, http.MethodGet, url, nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, string(b))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func contains(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}
