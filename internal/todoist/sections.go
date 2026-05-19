package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetSections(ctx context.Context, projectID string) ([]Section, error) {
	params := url.Values{}
	if projectID != "" {
		params.Set("project_id", projectID)
	}
	return queryAll[Section](c, ctx, "/sections", params)
}
