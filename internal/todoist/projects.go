package todoist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	return queryAll[Project](c, ctx, "/projects", url.Values{})
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var p Project
	return &p, c.doJSON(ctx, "GET", "/projects/"+id, nil, &p)
}

func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	var p Project
	return &p, c.doJSON(ctx, "POST", "/projects", req, &p)
}

func (c *Client) UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) (*Project, error) {
	var p Project
	return &p, c.doJSON(ctx, "POST", "/projects/"+id, req, &p)
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/projects/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// MoveProjectToParent reparents a project. The REST project endpoints have no
// move operation and the update endpoint rejects parent_id, so this goes
// through the Sync API's project_move command. An empty parentID promotes the
// project to the top level. The HTTP request can succeed while the command
// itself fails, so the per-command result in sync_status is checked too.
func (c *Client) MoveProjectToParent(ctx context.Context, projectID, parentID string) error {
	var pid any // nil (JSON null) moves the project to the top level
	if parentID != "" {
		pid = parentID
	}
	cmdUUID := uuid.NewString()
	commands, err := json.Marshal([]map[string]any{{
		"type": "project_move",
		"uuid": cmdUUID,
		"args": map[string]any{"id": projectID, "parent_id": pid},
	}})
	if err != nil {
		return err
	}

	form := url.Values{"commands": {string(commands)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiBaseURL()+"/sync", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return newAPIError(resp.StatusCode, body)
	}

	var out struct {
		SyncStatus map[string]json.RawMessage `json:"sync_status"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("todoist: unexpected sync response: %s", body)
	}
	status, ok := out.SyncStatus[cmdUUID]
	if !ok {
		return fmt.Errorf("todoist: sync response missing status for move command")
	}
	// A successful command reports the JSON string "ok"; a failure reports an
	// error object (e.g. {"error":"Invalid parent",...}).
	var okStr string
	if json.Unmarshal(status, &okStr) == nil && okStr == "ok" {
		return nil
	}
	return fmt.Errorf("todoist: project move rejected: %s", status)
}
