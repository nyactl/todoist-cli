package todoist

import (
	"context"
	"net/url"
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

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/projects/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
