package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetSections(ctx context.Context) ([]Section, error) {
	return queryAll[Section](c, ctx, "/sections", url.Values{})
}
