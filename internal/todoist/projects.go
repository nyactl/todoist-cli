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
