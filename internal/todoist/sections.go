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

func (c *Client) DeleteSection(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/sections/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
